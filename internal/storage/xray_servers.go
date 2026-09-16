package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// XrayServer represents a local or remote sing-box server.
// Table name xray_servers is kept for mmwX DB compatibility; the actual
// proxy kernel is sing-box (not Xray).
type XrayServer struct {
	ID                int64     `json:"id"`
	Name              string    `json:"name"`
	Host              string    `json:"host"`
	Port              int       `json:"port"`              // SSH port
	Description       string    `json:"description"`
	IsLocal           bool      `json:"is_local"`
	IsPrimary         bool      `json:"is_primary"`
	ProcessID         int       `json:"process_id"`
	ConfigPath        string    `json:"config_path"`       // remote config file path
	APPort            int       `json:"api_port"`          // sing-box clash API port
	SingboxPort       int       `json:"singbox_port"`      // inbound listen port
	SingboxUser       string    `json:"singbox_user"`      // optional inbound auth user
	SSHUser           string    `json:"ssh_user"`
	AuthType          string    `json:"auth_type"`         // "password" | "private_key"
	AuthData          string    `json:"-"`                  // password or PEM key (never returned to frontend)
	Enabled           bool      `json:"enabled"`
	HasStatus         string    `json:"has_status"`        // last known status message
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	TrafficLimit      int64     `json:"traffic_limit"`
	TrafficResetDay   int       `json:"traffic_reset_day"`
	TrafficUsedOffset int64     `json:"traffic_used_offset"`
}

// XrayServerAuth holds sensitive credentials, read separately when needed.
type XrayServerAuth struct {
	AuthType string
	AuthData string
}

const xrayServersSchema = `
CREATE TABLE IF NOT EXISTS xray_servers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    host TEXT NOT NULL,
    port INTEGER NOT NULL,
    description TEXT,
    is_local INTEGER NOT NULL DEFAULT 0,
    is_primary INTEGER NOT NULL DEFAULT 0,
    process_id INTEGER NOT NULL DEFAULT 0,
    config_path TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    traffic_limit INTEGER NOT NULL DEFAULT 0,
    traffic_reset_day INTEGER NOT NULL DEFAULT 0,
    traffic_used_offset INTEGER NOT NULL DEFAULT 0,
    UNIQUE(host, port)
);
`

func (r *TrafficRepository) migrateXrayServers() error {
	if _, err := r.db.Exec(xrayServersSchema); err != nil {
		return fmt.Errorf("migrate xray_servers: %w", err)
	}
	// Incremental columns for SSH-based sing-box management (mmwx-pro extension)
	incremental := []struct{ name, def string }{
		{"ssh_user", "TEXT NOT NULL DEFAULT 'root'"},
		{"auth_type", "TEXT NOT NULL DEFAULT 'password'"},
		{"auth_data", "TEXT NOT NULL DEFAULT ''"},
		{"api_port", "INTEGER NOT NULL DEFAULT 9090"},
		{"singbox_port", "INTEGER NOT NULL DEFAULT 7890"},
		{"singbox_user", "TEXT NOT NULL DEFAULT ''"},
		{"enabled", "INTEGER NOT NULL DEFAULT 1"},
		{"has_status", "TEXT NOT NULL DEFAULT ''"},
	}
	for _, col := range incremental {
		if err := r.ensureXrayServerColumn(col.name, col.def); err != nil {
			return err
		}
	}
	return nil
}

func (r *TrafficRepository) ensureXrayServerColumn(name, definition string) error {
	rows, err := r.db.Query(`PRAGMA table_info(xray_servers)`)
	if err != nil {
		return fmt.Errorf("xray_servers table info: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var colName, colType string
		var notNull int
		var defaultVal sql.NullString
		var pk int
		if err := rows.Scan(&cid, &colName, &colType, &notNull, &defaultVal, &pk); err != nil {
			return fmt.Errorf("scan table info: %w", err)
		}
		if equalFold(colName, name) {
			return nil
		}
	}
	alter := fmt.Sprintf("ALTER TABLE xray_servers ADD COLUMN %s %s", name, definition)
	if _, err := r.db.Exec(alter); err != nil {
		return fmt.Errorf("add column %s to xray_servers: %w", name, err)
	}
	return nil
}

var ErrXrayServerNotFound = errors.New("server not found")
var ErrXrayServerExists = errors.New("server name already exists")

// xrayServerColumns lists columns returned to the frontend (excludes auth_data).
func xrayServerColumns() string {
	return `id, name, host, port, COALESCE(description,''), is_local, is_primary, process_id, COALESCE(config_path,''), api_port, singbox_port, singbox_user, ssh_user, auth_type, enabled, COALESCE(has_status,''), created_at, updated_at, traffic_limit, traffic_reset_day, traffic_used_offset`
}

func scanXrayServer(row interface{ Scan(...any) error }) (XrayServer, error) {
	var s XrayServer
	var isLocal, isPrimary, enabled int
	if err := row.Scan(&s.ID, &s.Name, &s.Host, &s.Port, &s.Description, &isLocal, &isPrimary, &s.ProcessID, &s.ConfigPath, &s.APPort, &s.SingboxPort, &s.SingboxUser, &s.SSHUser, &s.AuthType, &enabled, &s.HasStatus, &s.CreatedAt, &s.UpdatedAt, &s.TrafficLimit, &s.TrafficResetDay, &s.TrafficUsedOffset); err != nil {
		return s, err
	}
	s.IsLocal = isLocal != 0
	s.IsPrimary = isPrimary != 0
	s.Enabled = enabled != 0
	return s, nil
}

// GetXrayServerAuth returns the sensitive auth fields (called by handler for SSH ops only).
func (r *TrafficRepository) GetXrayServerAuth(ctx context.Context, id int64) (XrayServerAuth, error) {
	var auth XrayServerAuth
	if id <= 0 {
		return auth, errors.New("server id is required")
	}
	err := r.db.QueryRowContext(ctx, `SELECT auth_type, auth_data FROM xray_servers WHERE id = ? LIMIT 1`, id).Scan(&auth.AuthType, &auth.AuthData)
	if errors.Is(err, sql.ErrNoRows) {
		return auth, ErrXrayServerNotFound
	}
	return auth, err
}

// SetXrayServerStatus updates the last known status string.
func (r *TrafficRepository) SetXrayServerStatus(ctx context.Context, id int64, status string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE xray_servers SET has_status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, status, id)
	return err
}

func (r *TrafficRepository) CreateXrayServer(ctx context.Context, s *XrayServer, auth *XrayServerAuth) (XrayServer, error) {
	if r == nil || r.db == nil {
		return XrayServer{}, errors.New("traffic repository not initialized")
	}
	s.Name = strings.TrimSpace(s.Name)
	s.Host = strings.TrimSpace(s.Host)
	if s.Name == "" {
		return XrayServer{}, errors.New("server name is required")
	}
	if s.Host == "" {
		return XrayServer{}, errors.New("server host is required")
	}
	if s.Port <= 0 {
		s.Port = 22
	}
	if s.SSHUser = strings.TrimSpace(s.SSHUser); s.SSHUser == "" {
		s.SSHUser = "root"
	}
	if s.ConfigPath = strings.TrimSpace(s.ConfigPath); s.ConfigPath == "" {
		s.ConfigPath = "/etc/sing-box/config.json"
	}
	if s.APPort <= 0 {
		s.APPort = 9090
	}
	if s.SingboxPort <= 0 {
		s.SingboxPort = 7890
	}
	if auth == nil {
		auth = &XrayServerAuth{AuthType: "password"}
	}
	if auth.AuthType == "" {
		auth.AuthType = "password"
	}
	if auth.AuthType != "password" && auth.AuthType != "private_key" {
		return XrayServer{}, errors.New("invalid auth type")
	}
	if auth.AuthData == "" {
		return XrayServer{}, errors.New("auth data is required")
	}

	res, err := r.db.ExecContext(ctx,
		`INSERT INTO xray_servers (name, host, port, description, is_local, is_primary, config_path, traffic_limit, traffic_reset_day, traffic_used_offset, ssh_user, auth_type, auth_data, api_port, singbox_port, singbox_user, enabled, has_status) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '')`,
		s.Name, s.Host, s.Port, s.Description, boolToIntXray(s.IsLocal), boolToIntXray(s.IsPrimary), s.ConfigPath,
		s.TrafficLimit, s.TrafficResetDay, s.TrafficUsedOffset,
		s.SSHUser, auth.AuthType, auth.AuthData, s.APPort, s.SingboxPort, s.SingboxUser, boolToIntXray(s.Enabled))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return XrayServer{}, fmt.Errorf("%w: %s", ErrXrayServerExists, err.Error())
		}
		return XrayServer{}, fmt.Errorf("create server: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return XrayServer{}, fmt.Errorf("fetch server id: %w", err)
	}
	return r.GetXrayServer(ctx, id)
}

func (r *TrafficRepository) ListXrayServers(ctx context.Context) ([]XrayServer, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+xrayServerColumns()+` FROM xray_servers ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list servers: %w", err)
	}
	defer rows.Close()
	var out []XrayServer
	for rows.Next() {
		s, err := scanXrayServer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *TrafficRepository) GetXrayServer(ctx context.Context, id int64) (XrayServer, error) {
	if id <= 0 {
		return XrayServer{}, errors.New("server id is required")
	}
	row := r.db.QueryRowContext(ctx, `SELECT `+xrayServerColumns()+` FROM xray_servers WHERE id = ? LIMIT 1`, id)
	s, err := scanXrayServer(row)
	if errors.Is(err, sql.ErrNoRows) {
		return s, ErrXrayServerNotFound
	}
	return s, err
}

// UpdateXrayServer updates a server. If auth is nil, auth fields stay unchanged.
func (r *TrafficRepository) UpdateXrayServer(ctx context.Context, s *XrayServer, auth *XrayServerAuth) (XrayServer, error) {
	if r == nil || r.db == nil {
		return XrayServer{}, errors.New("traffic repository not initialized")
	}
	if s.ID <= 0 {
		return XrayServer{}, errors.New("server id is required")
	}
	s.Name = strings.TrimSpace(s.Name)
	s.Host = strings.TrimSpace(s.Host)
	if s.Name == "" {
		return XrayServer{}, errors.New("server name is required")
	}
	if s.Host == "" {
		return XrayServer{}, errors.New("server host is required")
	}
	if s.Port <= 0 {
		s.Port = 22
	}
	if s.SSHUser = strings.TrimSpace(s.SSHUser); s.SSHUser == "" {
		s.SSHUser = "root"
	}
	if s.ConfigPath = strings.TrimSpace(s.ConfigPath); s.ConfigPath == "" {
		s.ConfigPath = "/etc/sing-box/config.json"
	}
	if s.APPort <= 0 {
		s.APPort = 9090
	}
	if s.SingboxPort <= 0 {
		s.SingboxPort = 7890
	}

	var res sql.Result
	var err error
	if auth != nil && auth.AuthData != "" {
		if auth.AuthType == "" {
			auth.AuthType = "password"
		}
		res, err = r.db.ExecContext(ctx,
			`UPDATE xray_servers SET name=?, host=?, port=?, description=?, is_local=?, is_primary=?, config_path=?, traffic_limit=?, traffic_reset_day=?, traffic_used_offset=?, ssh_user=?, auth_type=?, auth_data=?, api_port=?, singbox_port=?, singbox_user=?, enabled=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
			s.Name, s.Host, s.Port, s.Description, boolToIntXray(s.IsLocal), boolToIntXray(s.IsPrimary), s.ConfigPath,
			s.TrafficLimit, s.TrafficResetDay, s.TrafficUsedOffset,
			s.SSHUser, auth.AuthType, auth.AuthData, s.APPort, s.SingboxPort, s.SingboxUser, boolToIntXray(s.Enabled), s.ID)
	} else {
		res, err = r.db.ExecContext(ctx,
			`UPDATE xray_servers SET name=?, host=?, port=?, description=?, is_local=?, is_primary=?, config_path=?, traffic_limit=?, traffic_reset_day=?, traffic_used_offset=?, ssh_user=?, api_port=?, singbox_port=?, singbox_user=?, enabled=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
			s.Name, s.Host, s.Port, s.Description, boolToIntXray(s.IsLocal), boolToIntXray(s.IsPrimary), s.ConfigPath,
			s.TrafficLimit, s.TrafficResetDay, s.TrafficUsedOffset,
			s.SSHUser, s.APPort, s.SingboxPort, s.SingboxUser, boolToIntXray(s.Enabled), s.ID)
	}
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return XrayServer{}, fmt.Errorf("%w: %s", ErrXrayServerExists, err.Error())
		}
		return XrayServer{}, fmt.Errorf("update server: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return XrayServer{}, ErrXrayServerNotFound
	}
	return r.GetXrayServer(ctx, s.ID)
}

func (r *TrafficRepository) DeleteXrayServer(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM xray_servers WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete server: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrXrayServerNotFound
	}
	return nil
}

// --- NodeTraffic (per-inbound/outbound traffic tracking) ---

type NodeTraffic struct {
	ID            int64     `json:"id"`
	ServerID      int64     `json:"server_id"`
	Tag           string    `json:"tag"`
	Type          string    `json:"type"` // inbound | outbound
	Uplink        int64     `json:"uplink"`
	Downlink      int64     `json:"downlink"`
	TotalUplink   int64     `json:"total_uplink"`
	TotalDownlink int64     `json:"total_downlink"`
	LastUplink    int64     `json:"last_uplink"`
	LastDownlink  int64     `json:"last_downlink"`
	UpdatedAt     time.Time `json:"updated_at"`
}

const nodeTrafficSchema = `
CREATE TABLE IF NOT EXISTS node_traffic (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id INTEGER NOT NULL,
    tag TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('inbound', 'outbound')),
    uplink INTEGER NOT NULL DEFAULT 0,
    downlink INTEGER NOT NULL DEFAULT 0,
    total_uplink INTEGER NOT NULL DEFAULT 0,
    total_downlink INTEGER NOT NULL DEFAULT 0,
    last_uplink INTEGER NOT NULL DEFAULT 0,
    last_downlink INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(server_id, tag, type),
    FOREIGN KEY (server_id) REFERENCES xray_servers(id) ON DELETE CASCADE
);
`

func (r *TrafficRepository) migrateNodeTraffic() error {
	_, err := r.db.Exec(nodeTrafficSchema)
	return err
}

// --- UserTraffic (per-user traffic on a server) ---

type UserTraffic struct {
	ID            int64     `json:"id"`
	ServerID      int64     `json:"server_id"`
	Username      string    `json:"username"`
	Uplink        int64     `json:"uplink"`
	Downlink      int64     `json:"downlink"`
	TotalUplink   int64     `json:"total_uplink"`
	TotalDownlink int64     `json:"total_downlink"`
	LastUplink    int64     `json:"last_uplink"`
	LastDownlink  int64     `json:"last_downlink"`
	CycleStart    time.Time `json:"cycle_start"`
	UpdatedAt     time.Time `json:"updated_at"`
}

const userTrafficSchema = `
CREATE TABLE IF NOT EXISTS user_traffic (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id INTEGER NOT NULL,
    username TEXT NOT NULL,
    uplink INTEGER NOT NULL DEFAULT 0,
    downlink INTEGER NOT NULL DEFAULT 0,
    total_uplink INTEGER NOT NULL DEFAULT 0,
    total_downlink INTEGER NOT NULL DEFAULT 0,
    last_uplink INTEGER NOT NULL DEFAULT 0,
    last_downlink INTEGER NOT NULL DEFAULT 0,
    cycle_start TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(server_id, username),
    FOREIGN KEY (server_id) REFERENCES xray_servers(id) ON DELETE CASCADE
);
`

func (r *TrafficRepository) migrateUserTraffic() error {
	_, err := r.db.Exec(userTrafficSchema)
	return err
}

// --- UserEmailTraffic (per-email traffic on a server) ---

type UserEmailTraffic struct {
	ID            int64     `json:"id"`
	ServerID      int64     `json:"server_id"`
	Email         string    `json:"email"`
	Uplink        int64     `json:"uplink"`
	Downlink      int64     `json:"downlink"`
	TotalUplink   int64     `json:"total_uplink"`
	TotalDownlink int64     `json:"total_downlink"`
	LastUplink    int64     `json:"last_uplink"`
	LastDownlink  int64     `json:"last_downlink"`
	CycleStart    time.Time `json:"cycle_start"`
	UpdatedAt     time.Time `json:"updated_at"`
}

const userEmailTrafficSchema = `
CREATE TABLE IF NOT EXISTS user_email_traffic (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id INTEGER NOT NULL,
    email TEXT NOT NULL,
    uplink INTEGER NOT NULL DEFAULT 0,
    downlink INTEGER NOT NULL DEFAULT 0,
    total_uplink INTEGER NOT NULL DEFAULT 0,
    total_downlink INTEGER NOT NULL DEFAULT 0,
    last_uplink INTEGER NOT NULL DEFAULT 0,
    last_downlink INTEGER NOT NULL DEFAULT 0,
    cycle_start TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(server_id, email),
    FOREIGN KEY (server_id) REFERENCES xray_servers(id) ON DELETE CASCADE
);
`

func (r *TrafficRepository) migrateUserEmailTraffic() error {
	_, err := r.db.Exec(userEmailTrafficSchema)
	return err
}

// --- TrafficSnapshots (daily server traffic snapshots) ---

type TrafficSnapshot struct {
	ID                    int64     `json:"id"`
	ServerID              int64     `json:"server_id"`
	Date                  string    `json:"date"`
	InboundUplink         int64     `json:"inbound_uplink"`
	InboundDownlink       int64     `json:"inbound_downlink"`
	OutboundUplink        int64     `json:"outbound_uplink"`
	OutboundDownlink      int64     `json:"outbound_downlink"`
	UserUplink            int64     `json:"user_uplink"`
	UserDownlink          int64     `json:"user_downlink"`
	CreatedAt             time.Time `json:"created_at"`
}

const trafficSnapshotsSchema = `
CREATE TABLE IF NOT EXISTS traffic_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id INTEGER NOT NULL,
    date TEXT NOT NULL,
    inbound_uplink INTEGER NOT NULL DEFAULT 0,
    inbound_downlink INTEGER NOT NULL DEFAULT 0,
    outbound_uplink INTEGER NOT NULL DEFAULT 0,
    outbound_downlink INTEGER NOT NULL DEFAULT 0,
    user_uplink INTEGER NOT NULL DEFAULT 0,
    user_downlink INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(server_id, date),
    FOREIGN KEY (server_id) REFERENCES xray_servers(id) ON DELETE CASCADE
);
`

func (r *TrafficRepository) migrateTrafficSnapshots() error {
	_, err := r.db.Exec(trafficSnapshotsSchema)
	return err
}

// --- BatchInbounds / BatchOutbounds ---

type BatchInbound struct {
	ID        int64     `json:"id"`
	BatchID   string    `json:"batch_id"`
	Tag       string    `json:"tag"`
	ServerID  int64     `json:"server_id"`
	Protocol  string    `json:"protocol"`
	Port      int       `json:"port"`
	CreatedAt time.Time `json:"created_at"`
}

const batchInboundsSchema = `
CREATE TABLE IF NOT EXISTS batch_inbounds (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    batch_id TEXT NOT NULL,
    tag TEXT NOT NULL,
    server_id INTEGER NOT NULL,
    protocol TEXT NOT NULL,
    port INTEGER NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (server_id) REFERENCES xray_servers(id) ON DELETE CASCADE
);
`

func (r *TrafficRepository) migrateBatchInbounds() error {
	_, err := r.db.Exec(batchInboundsSchema)
	return err
}

type BatchOutbound struct {
	ID        int64     `json:"id"`
	BatchID   string    `json:"batch_id"`
	Tag       string    `json:"tag"`
	ServerID  int64     `json:"server_id"`
	Protocol  string    `json:"protocol"`
	CreatedAt time.Time `json:"created_at"`
}

const batchOutboundsSchema = `
CREATE TABLE IF NOT EXISTS batch_outbounds (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    batch_id TEXT NOT NULL,
    tag TEXT NOT NULL,
    server_id INTEGER NOT NULL,
    protocol TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (server_id) REFERENCES xray_servers(id) ON DELETE CASCADE
);
`

func (r *TrafficRepository) migrateBatchOutbounds() error {
	_, err := r.db.Exec(batchOutboundsSchema)
	return err
}

// --- UserInboundConfigs / UserOutbounds ---

type UserInboundConfig struct {
	ID              int64     `json:"id"`
	Username        string    `json:"username"`
	ServerID        int64     `json:"server_id"`
	InboundTag      string    `json:"inbound_tag"`
	Protocol        string    `json:"protocol"`
	CredentialJSON  string    `json:"credential_json"`
	CreatedAt       time.Time `json:"created_at"`
}

const userInboundConfigsSchema = `
CREATE TABLE IF NOT EXISTS user_inbound_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL,
    server_id INTEGER NOT NULL,
    inbound_tag TEXT NOT NULL,
    protocol TEXT NOT NULL,
    credential_json TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

func (r *TrafficRepository) migrateUserInboundConfigs() error {
	_, err := r.db.Exec(userInboundConfigsSchema)
	return err
}

type UserOutbound struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	ServerID     int64     `json:"server_id"`
	InboundTag   string    `json:"inbound_tag"`
	OutboundTag  string    `json:"outbound_tag"`
	OutboundJSON string    `json:"outbound_json"`
	CreatedAt    time.Time `json:"created_at"`
}

const userOutboundsSchema = `
CREATE TABLE IF NOT EXISTS user_outbounds (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL,
    server_id INTEGER NOT NULL,
    inbound_tag TEXT NOT NULL,
    outbound_tag TEXT NOT NULL,
    outbound_json TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(username) REFERENCES users(username) ON DELETE CASCADE
);
`

func (r *TrafficRepository) migrateUserOutbounds() error {
	_, err := r.db.Exec(userOutboundsSchema)
	return err
}

func boolToIntXray(b bool) int {
	if b {
		return 1
	}
	return 0
}

// SyncServerNode creates or updates a node in the nodes table for the given xray_server.
// The node represents the server's mixed inbound as a Clash proxy.
// username specifies which user owns the synced node.
// clashConfig is the pre-built Clash proxy JSON string to store as the node's clash_config.
func (r *TrafficRepository) SyncServerNode(ctx context.Context, serverID int64, username, clashConfig string) (Node, error) {
	if r == nil || r.db == nil {
		return Node{}, errors.New("traffic repository not initialized")
	}
	if serverID <= 0 {
		return Node{}, errors.New("server id is required")
	}
	if username == "" {
		return Node{}, errors.New("username is required")
	}
	if clashConfig == "" {
		return Node{}, errors.New("clash config is required")
	}

	// Get the server to build node name / protocol
	srv, err := r.GetXrayServer(ctx, serverID)
	if err != nil {
		return Node{}, err
	}

	nodeName := srv.Name
	protocol := "mixed"
	parsedConfig := fmt.Sprintf(`{"server":"%s","server_port":%d}`, srv.Host, srv.SingboxPort)

	// Check if a node already exists for this singbox_server_id
	var existingID int64
	err = r.db.QueryRowContext(ctx, `SELECT id FROM nodes WHERE singbox_server_id = ? AND username = ? LIMIT 1`, serverID, username).Scan(&existingID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Node{}, fmt.Errorf("query existing server node: %w", err)
	}

	if existingID > 0 {
		_, err := r.db.ExecContext(ctx, `UPDATE nodes SET node_name = ?, protocol = ?, parsed_config = ?, clash_config = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND username = ?`, nodeName, protocol, parsedConfig, clashConfig, existingID, username)
		if err != nil {
			return Node{}, fmt.Errorf("update server node: %w", err)
		}
		return r.GetNode(ctx, existingID, username)
	}

	res, err := r.db.ExecContext(ctx, `INSERT INTO nodes (username, raw_url, node_name, protocol, parsed_config, clash_config, enabled, tag, tags, singbox_server_id) VALUES (?, '', ?, ?, ?, ?, 1, 'sing-box', '["sing-box"]', ?)`, username, nodeName, protocol, parsedConfig, clashConfig, serverID)
	if err != nil {
		return Node{}, fmt.Errorf("create server node: %w", err)
	}
	newID, err := res.LastInsertId()
	if err != nil {
		return Node{}, fmt.Errorf("fetch server node id: %w", err)
	}
	return r.GetNode(ctx, newID, username)
}

