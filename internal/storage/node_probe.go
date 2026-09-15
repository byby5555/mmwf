package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ProbeConfig 探测配置（mmwx_clean 独有，用于 traffic_summary handler）
type ProbeConfig struct {
	ID        int64
	ProbeType string
	Address   string
	Servers   []ProbeServer
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ProbeServer 探测服务器（mmwx_clean 独有）
type ProbeServer struct {
	ID                  int64
	ConfigID            int64
	ServerID            string
	Name                string
	TrafficMethod       string
	MonthlyTrafficBytes int64
	Position            int
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// 外部节点连通性探测的数据层。
//
// 探测本身很重 —— 每个节点要起一个 mihomo 进程真连一次 —— 所以不对全部导入节点跑,
// 由用户在节点列表逐个勾选(probe_enabled)。这里只提供「取待探测清单」和「改开关」,
// 探测结果不落库:默认存内存 ring(见 handler 侧),避免重蹈 forward_hop_metrics 的覆辙
// (分钟级时序把 SQLite 的 WAL 顶到几十 GB,而主库才十几 MB)。

// ProbeTargetNode 一个待探测节点的最小信息。
// 刻意不返回整个 Node:探测只需要这几样,而 clash_config 可能很大,
// 一轮探测几百个节点时没必要把无关字段全读进内存。
type ProbeTargetNode struct {
	ID       int64
	NodeName string
	Protocol string
	// Username 节点归属用户。「掉线自动重新同步外部订阅」按用户维度同步
	// (syncExternalSubscriptions 就是按 username 拉该用户的全部外部订阅)。
	Username    string
	ClashConfig string
}

// ListProbeEnabledNodes 取所有勾选了探测的节点。
//
// 不按 original_server 过滤 —— 妙妙屋X 那边用 original_server=” 筛「外部节点」,
// 因为它那里该列存的是自建服务器名。**本项目里 original_server 语义完全不同**:
// 它是「手动改节点服务器地址时备份的旧地址」(见 handler/nodes.go handleUpdateServer),
// 照抄过来会把所有手动改过地址的节点莫名排除掉。
//
// 而且本项目没有「自建服务器」这个概念,节点全都是导入来的,本来就都该可探测。
// 真正的开关是用户逐个勾选的 probe_enabled。
func (r *TrafficRepository) ListProbeEnabledNodes(ctx context.Context) ([]ProbeTargetNode, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("traffic repository not initialized")
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, node_name, protocol, username, clash_config
		FROM nodes
		WHERE COALESCE(probe_enabled, 0) = 1
		  AND enabled = 1
		ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list probe-enabled nodes: %w", err)
	}
	defer rows.Close()

	var out []ProbeTargetNode
	for rows.Next() {
		var n ProbeTargetNode
		if err := rows.Scan(&n.ID, &n.NodeName, &n.Protocol, &n.Username, &n.ClashConfig); err != nil {
			return nil, fmt.Errorf("scan probe target: %w", err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// SetNodeProbeEnabled 开关某个节点的探测。
func (r *TrafficRepository) SetNodeProbeEnabled(ctx context.Context, nodeID int64, enabled bool) error {
	if r == nil || r.db == nil {
		return errors.New("traffic repository not initialized")
	}
	v := 0
	if enabled {
		v = 1
	}
	res, err := r.db.ExecContext(ctx,
		`UPDATE nodes SET probe_enabled = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, v, nodeID)
	if err != nil {
		return fmt.Errorf("set node probe_enabled: %w", err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return fmt.Errorf("节点 %d 不存在", nodeID)
	}
	return nil
}

// CountProbeEnabledNodes 供前端展示"已开启探测 N 个",也用于调度器空转时提前返回。
func (r *TrafficRepository) CountProbeEnabledNodes(ctx context.Context) (int, error) {
	if r == nil || r.db == nil {
		return 0, errors.New("traffic repository not initialized")
	}
	var n int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM nodes
		WHERE COALESCE(probe_enabled, 0) = 1 AND enabled = 1
`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count probe-enabled nodes: %w", err)
	}
	return n, nil
}

// ---- probe_configs / probe_servers 表与 CRUD ----

func scanProbeConfig(scanner rowScanner) (ProbeConfig, error) {
	var cfg ProbeConfig
	if err := scanner.Scan(&cfg.ID, &cfg.ProbeType, &cfg.Address, &cfg.CreatedAt, &cfg.UpdatedAt); err != nil {
		return ProbeConfig{}, err
	}
	return cfg, nil
}

func scanProbeServer(scanner rowScanner) (ProbeServer, error) {
	var srv ProbeServer
	if err := scanner.Scan(&srv.ID, &srv.ConfigID, &srv.ServerID, &srv.Name, &srv.TrafficMethod, &srv.MonthlyTrafficBytes, &srv.Position, &srv.CreatedAt, &srv.UpdatedAt); err != nil {
		return ProbeServer{}, err
	}
	return srv, nil
}

// migrateProbeTables 创建 probe_configs 和 probe_servers 表。
func (r *TrafficRepository) migrateProbeTables() error {
	const schema = `
CREATE TABLE IF NOT EXISTS probe_configs (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    probe_type TEXT NOT NULL CHECK (probe_type IN ('nezha','nezhav0','dstatus','komari')),
    address TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS probe_servers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    config_id INTEGER NOT NULL,
    server_id TEXT NOT NULL,
    name TEXT NOT NULL,
    traffic_method TEXT NOT NULL CHECK (traffic_method IN ('up','down','both')),
    monthly_traffic_bytes INTEGER NOT NULL DEFAULT 0 CHECK (monthly_traffic_bytes >= 0),
    position INTEGER NOT NULL DEFAULT 0 CHECK (position >= 0),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(config_id) REFERENCES probe_configs(id) ON DELETE CASCADE,
    UNIQUE(config_id, server_id)
);
`
	if _, err := r.db.Exec(schema); err != nil {
		return fmt.Errorf("migrate probe tables: %w", err)
	}
	return nil
}

// GetProbeConfig returns the current probe configuration with associated servers.
func (r *TrafficRepository) GetProbeConfig(ctx context.Context) (ProbeConfig, error) {
	var cfg ProbeConfig
	if r == nil || r.db == nil {
		return cfg, errors.New("traffic repository not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	row := r.db.QueryRowContext(ctx, `SELECT id, probe_type, address, created_at, updated_at FROM probe_configs WHERE id = 1 LIMIT 1`)
	result, err := scanProbeConfig(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return cfg, ErrProbeConfigNotFound
		}
		return cfg, fmt.Errorf("get probe config: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, `SELECT id, config_id, server_id, name, traffic_method, monthly_traffic_bytes, position, created_at, updated_at FROM probe_servers WHERE config_id = ? ORDER BY position ASC, id ASC`, result.ID)
	if err != nil {
		return cfg, fmt.Errorf("list probe servers: %w", err)
	}
	defer rows.Close()

	var servers []ProbeServer
	for rows.Next() {
		server, err := scanProbeServer(rows)
		if err != nil {
			return cfg, fmt.Errorf("scan probe server: %w", err)
		}
		servers = append(servers, server)
	}

	if err := rows.Err(); err != nil {
		return cfg, fmt.Errorf("iterate probe servers: %w", err)
	}

	result.Servers = servers

	return result, nil
}

// UpsertProbeConfig updates the singleton probe configuration and replaces its server list.
func (r *TrafficRepository) UpsertProbeConfig(ctx context.Context, cfg ProbeConfig) (ProbeConfig, error) {
	if r == nil || r.db == nil {
		return ProbeConfig{}, errors.New("traffic repository not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	cfg.ProbeType = strings.ToLower(strings.TrimSpace(cfg.ProbeType))
	if _, ok := allowedProbeTypes[cfg.ProbeType]; !ok {
		return ProbeConfig{}, errors.New("unsupported probe type")
	}

	cfg.Address = strings.TrimSpace(cfg.Address)
	if cfg.Address == "" {
		return ProbeConfig{}, errors.New("probe address is required")
	}

	if len(cfg.Servers) == 0 {
		return ProbeConfig{}, errors.New("at least one server is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ProbeConfig{}, fmt.Errorf("begin probe config tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `INSERT INTO probe_configs (id, probe_type, address) VALUES (1, ?, ?) ON CONFLICT(id) DO UPDATE SET probe_type = excluded.probe_type, address = excluded.address, updated_at = CURRENT_TIMESTAMP`, cfg.ProbeType, cfg.Address); err != nil {
		return ProbeConfig{}, fmt.Errorf("upsert probe config: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM probe_servers WHERE config_id = 1`); err != nil {
		return ProbeConfig{}, fmt.Errorf("clear probe servers: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO probe_servers (config_id, server_id, name, traffic_method, monthly_traffic_bytes, position) VALUES (1, ?, ?, ?, ?, ?)`)
	if err != nil {
		return ProbeConfig{}, fmt.Errorf("prepare insert probe server: %w", err)
	}
	defer stmt.Close()

	for idx, srv := range cfg.Servers {
		serverID := strings.TrimSpace(srv.ServerID)
		if serverID == "" {
			return ProbeConfig{}, fmt.Errorf("server %d: server id is required", idx+1)
		}
		name := strings.TrimSpace(srv.Name)
		if name == "" {
			return ProbeConfig{}, fmt.Errorf("server %d: server name is required", idx+1)
		}
		method := strings.ToLower(strings.TrimSpace(srv.TrafficMethod))
		if _, ok := allowedTrafficMethods[method]; !ok {
			return ProbeConfig{}, fmt.Errorf("server %d: unsupported traffic method", idx+1)
		}
		if srv.MonthlyTrafficBytes < 0 {
			return ProbeConfig{}, fmt.Errorf("server %d: monthly traffic cannot be negative", idx+1)
		}
		if _, err := stmt.ExecContext(ctx, serverID, name, method, srv.MonthlyTrafficBytes, idx); err != nil {
			return ProbeConfig{}, fmt.Errorf("insert probe server %d: %w", idx+1, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return ProbeConfig{}, fmt.Errorf("commit probe config: %w", err)
	}

	return r.GetProbeConfig(ctx)
}

// DeleteProbeConfig deletes the probe configuration and clears all node probe bindings.
func (r *TrafficRepository) DeleteProbeConfig(ctx context.Context) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete probe config tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `UPDATE nodes SET probe_server = '' WHERE probe_server != ''`); err != nil {
		return fmt.Errorf("clear node probe bindings: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM probe_servers WHERE config_id = 1`); err != nil {
		return fmt.Errorf("delete probe servers: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM probe_configs WHERE id = 1`); err != nil {
		return fmt.Errorf("delete probe config: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete probe config: %w", err)
	}

	return nil
}

// CleanupStatsServerIDs removes invalid server IDs from all subscribe files' stats_server_ids.
func (r *TrafficRepository) CleanupStatsServerIDs(ctx context.Context, validServerIDs []string) error {
	if r == nil || r.db == nil {
		return errors.New("traffic repository not initialized")
	}

	validSet := make(map[string]struct{}, len(validServerIDs))
	for _, id := range validServerIDs {
		validSet[id] = struct{}{}
	}

	rows, err := r.db.QueryContext(ctx, `SELECT id, stats_server_ids FROM subscribe_files WHERE stats_server_ids != ''`)
	if err != nil {
		return fmt.Errorf("query stats_server_ids: %w", err)
	}
	defer rows.Close()

	type record struct {
		id  int64
		ids string
	}
	var records []record
	for rows.Next() {
		var rec record
		if err := rows.Scan(&rec.id, &rec.ids); err != nil {
			return fmt.Errorf("scan stats_server_ids: %w", err)
		}
		records = append(records, rec)
	}

	for _, rec := range records {
		parts := strings.Split(rec.ids, ",")
		var kept []string
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if _, ok := validSet[p]; ok {
				kept = append(kept, p)
			}
		}
		newVal := strings.Join(kept, ",")
		if newVal != rec.ids {
			if _, err := r.db.ExecContext(ctx, `UPDATE subscribe_files SET stats_server_ids = ? WHERE id = ?`, newVal, rec.id); err != nil {
				return fmt.Errorf("update stats_server_ids for file %d: %w", rec.id, err)
			}
		}
	}

	return nil
}

// ClearAllStatsServerIDs resets stats_server_ids for all subscribe files.
func (r *TrafficRepository) ClearAllStatsServerIDs(ctx context.Context) error {
	if r == nil || r.db == nil {
		return errors.New("traffic repository not initialized")
	}
	_, err := r.db.ExecContext(ctx, `UPDATE subscribe_files SET stats_server_ids = '' WHERE stats_server_ids != ''`)
	if err != nil {
		return fmt.Errorf("clear all stats_server_ids: %w", err)
	}
	return nil
}
