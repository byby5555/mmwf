package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Package代表流量包模板
type Package struct {
	ID                int64             `json:"id"`
	Name              string            `json:"name"`
	Description       string            `json:"description"`
	TrafficLimitGB    float64           `json:"traffic_limit_gb"`           // GB 流量限制
	TrafficLimitBytes int64             `json:"-"`                          // 流量限制（以字节为单位）（仅限内部使用）
	CycleDays         int               `json:"cycle_days"`                 // 包裹持续时间（天）
	IsReset           bool              `json:"is_reset"`                   // 流量是否按月重置
	ResetDay          int               `json:"reset_day"`                  // 重置的月份日期 (1-31)
	Nodes             []int64           `json:"nodes"`                      // 关联节点 ID
	NodeMultipliers   map[int64]float64 `json:"node_multipliers,omitempty"` // node_id → 倍率;遗留套餐为 nil = 全部按 1
	SpeedLimitMbps    float64           `json:"speed_limit_mbps"`           // 限速 (Mbps)，0=不限
	DeviceLimit       int               `json:"device_limit"`               // 设备数限制，0=不限
	// 套餐级 per-node 限速覆盖。map 含 key 即生效:0 = 显式不限速,>0 = 该值;不含 key = 继承 SpeedLimitMbps。
	NodeSpeedLimits map[int64]float64 `json:"node_speed_limits,omitempty"`
	// 套餐级 per-node 客户端数覆盖。语义同上。
	NodeDeviceLimits map[int64]int        `json:"node_device_limits,omitempty"`
	AutoSpeedRules   []AutoSpeedLimitRule `json:"auto_speed_rules,omitempty"`
	ShortCode        string               `json:"short_code"`
	TrafficMode      string               `json:"traffic_mode"`
	TemplateFilename string               `json:"template_filename"` // 套餐绑的 V3 模板;空 = 走系统默认模板
	CreatedAt        time.Time            `json:"created_at"`
	UpdatedAt        time.Time            `json:"updated_at"`

	// mmwx_clean 独有新字段（当前业务代码未消费，仅定义用于前端兼容）
	NodeNameOverrides       string  `json:"node_name_overrides"`        // JSON object
	NodeNameOverrideEnabled bool    `json:"node_name_override_enabled"`
	SurgeTemplateFilename   string  `json:"surge_template_filename"`
	ForwardRuleLimit        int     `json:"forward_rule_limit"`
	ForwardPortLimit        int     `json:"forward_port_limit"`
	ForwardSpeedMbps        float64 `json:"forward_speed_mbps"`
	ForwardConnLimit        int     `json:"forward_conn_limit"`
	ForwardChains           string  `json:"forward_chains"` // JSON array
	NodeTrafficLimits       string  `json:"node_traffic_limits"` // JSON object
}

var ErrPackageNotFound = errors.New("package not found")

func (r *TrafficRepository) ListPackages(ctx context.Context) ([]Package, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("traffic repository not initialized")
	}

	const query = `
		SELECT id, name, COALESCE(description, ''), traffic_limit_bytes, cycle_days,
		       is_reset, reset_day, COALESCE(nodes, '[]'), COALESCE(speed_limit_mbps, 0), COALESCE(device_limit, 0),
		       COALESCE(auto_speed_limit_json, ''), COALESCE(short_code, ''), COALESCE(traffic_mode, 'oneway'), COALESCE(template_filename, ''), COALESCE(node_multipliers, '{}'), COALESCE(node_speed_limits, '{}'), COALESCE(node_device_limits, '{}'), created_at, updated_at
		FROM packages
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list packages: %w", err)
	}
	defer rows.Close()

	var packages []Package
	for rows.Next() {
		var pkg Package
		var isReset int
		var nodesJSON, autoSpeedJSON, nodeMultJSON, nodeSpeedJSON, nodeDeviceJSON string
		err := rows.Scan(&pkg.ID, &pkg.Name, &pkg.Description, &pkg.TrafficLimitBytes,
			&pkg.CycleDays, &isReset, &pkg.ResetDay, &nodesJSON, &pkg.SpeedLimitMbps, &pkg.DeviceLimit,
			&autoSpeedJSON, &pkg.ShortCode, &pkg.TrafficMode, &pkg.TemplateFilename, &nodeMultJSON, &nodeSpeedJSON, &nodeDeviceJSON, &pkg.CreatedAt, &pkg.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan package: %w", err)
		}
		pkg.IsReset = isReset != 0
		pkg.TrafficLimitGB = float64(pkg.TrafficLimitBytes) / (1024 * 1024 * 1024)

		pkg.Nodes = []int64{}
		if nodesJSON != "" && nodesJSON != "[]" {
			if err := json.Unmarshal([]byte(nodesJSON), &pkg.Nodes); err != nil {
				pkg.Nodes = []int64{}
			}
		}
		if autoSpeedJSON != "" {
			json.Unmarshal([]byte(autoSpeedJSON), &pkg.AutoSpeedRules)
		}
		if nodeMultJSON != "" && nodeMultJSON != "{}" {
			json.Unmarshal([]byte(nodeMultJSON), &pkg.NodeMultipliers)
		}
		if nodeSpeedJSON != "" && nodeSpeedJSON != "{}" {
			unmarshalStringKeyedMap(nodeSpeedJSON, &pkg.NodeSpeedLimits)
		}
		if nodeDeviceJSON != "" && nodeDeviceJSON != "{}" {
			unmarshalStringKeyedIntMap(nodeDeviceJSON, &pkg.NodeDeviceLimits)
		}

		packages = append(packages, pkg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate packages: %w", err)
	}

	// 静默过滤孤儿 node id
	idUnion := make([]int64, 0)
	seen := make(map[int64]bool)
	for _, pkg := range packages {
		for _, id := range pkg.Nodes {
			if !seen[id] {
				idUnion = append(idUnion, id)
				seen[id] = true
			}
		}
	}
	if len(idUnion) > 0 {
		if alive, err := r.aliveNodeIDs(ctx, idUnion); err == nil {
			for i := range packages {
				if len(packages[i].Nodes) == 0 {
					continue
				}
				out := make([]int64, 0, len(packages[i].Nodes))
				for _, id := range packages[i].Nodes {
					if alive[id] {
						out = append(out, id)
					}
				}
				packages[i].Nodes = out
			}
		}
	}

	return packages, nil
}

// 按 ID 返回包
func (r *TrafficRepository) GetPackage(ctx context.Context, id int64) (*Package, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("traffic repository not initialized")
	}

	const query = `
		SELECT id, name, COALESCE(description, ''), traffic_limit_bytes, cycle_days,
		       is_reset, reset_day, COALESCE(nodes, '[]'), COALESCE(speed_limit_mbps, 0), COALESCE(device_limit, 0),
		       COALESCE(auto_speed_limit_json, ''), COALESCE(short_code, ''), COALESCE(traffic_mode, 'oneway'), COALESCE(template_filename, ''), COALESCE(node_multipliers, '{}'), COALESCE(node_speed_limits, '{}'), COALESCE(node_device_limits, '{}'), created_at, updated_at
		FROM packages
		WHERE id = ?
	`

	var pkg Package
	var isReset int
	var nodesJSON, autoSpeedJSON, nodeMultJSON, nodeSpeedJSON, nodeDeviceJSON string
	err := r.db.QueryRowContext(ctx, query, id).Scan(&pkg.ID, &pkg.Name, &pkg.Description,
		&pkg.TrafficLimitBytes, &pkg.CycleDays, &isReset, &pkg.ResetDay, &nodesJSON,
		&pkg.SpeedLimitMbps, &pkg.DeviceLimit, &autoSpeedJSON, &pkg.ShortCode, &pkg.TrafficMode,
		&pkg.TemplateFilename, &nodeMultJSON, &nodeSpeedJSON, &nodeDeviceJSON, &pkg.CreatedAt, &pkg.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrPackageNotFound
		}
		return nil, fmt.Errorf("get package: %w", err)
	}

	pkg.IsReset = isReset != 0
	pkg.TrafficLimitGB = float64(pkg.TrafficLimitBytes) / (1024 * 1024 * 1024)

	pkg.Nodes = []int64{}
	if nodesJSON != "" && nodesJSON != "[]" {
		if err := json.Unmarshal([]byte(nodesJSON), &pkg.Nodes); err != nil {
			pkg.Nodes = []int64{}
		}
	}
	if autoSpeedJSON != "" {
		json.Unmarshal([]byte(autoSpeedJSON), &pkg.AutoSpeedRules)
	}
	if nodeMultJSON != "" && nodeMultJSON != "{}" {
		json.Unmarshal([]byte(nodeMultJSON), &pkg.NodeMultipliers)
	}
	if nodeSpeedJSON != "" && nodeSpeedJSON != "{}" {
		unmarshalStringKeyedMap(nodeSpeedJSON, &pkg.NodeSpeedLimits)
	}
	if nodeDeviceJSON != "" && nodeDeviceJSON != "{}" {
		unmarshalStringKeyedIntMap(nodeDeviceJSON, &pkg.NodeDeviceLimits)
	}

	// 静默过滤孤儿 node id
	pkg.Nodes = r.filterAliveNodeIDs(ctx, pkg.Nodes)

	return &pkg, nil
}

// 创建一个新的包模板
func (r *TrafficRepository) CreatePackage(ctx context.Context, pkg Package) (int64, error) {
	if r == nil || r.db == nil {
		return 0, errors.New("traffic repository not initialized")
	}

	name := strings.TrimSpace(pkg.Name)
	if name == "" {
		return 0, errors.New("package name is required")
	}

	// 检查同名的包是否已经存在
	if existing, err := r.GetPackageByName(ctx, name); err == nil && existing != nil {
		return 0, ErrPackageExists
	}

	// 将节点序列化为 JSON
	nodesJSON, err := json.Marshal(pkg.Nodes)
	if err != nil {
		return 0, fmt.Errorf("serialize nodes: %w", err)
	}

	var autoSpeedJSON string
	if len(pkg.AutoSpeedRules) > 0 {
		b, _ := json.Marshal(pkg.AutoSpeedRules)
		autoSpeedJSON = string(b)
	}

	// node_multipliers 序列化
	nodeMultJSON := serializeNodeMultipliers(pkg.NodeMultipliers, pkg.Nodes)
	// per-node 限速 / 客户端数
	nodeSpeedJSON := serializeNodeFloatMap(pkg.NodeSpeedLimits, pkg.Nodes)
	nodeDeviceJSON := serializeNodeIntMap(pkg.NodeDeviceLimits, pkg.Nodes)

	// 生成短码
	shortCode, err := generatePackageShortCode()
	if err != nil {
		return 0, err
	}

	const query = `
		INSERT INTO packages (name, description, traffic_limit_bytes, cycle_days, is_reset, reset_day, nodes, speed_limit_mbps, device_limit, auto_speed_limit_json, short_code, traffic_mode, template_filename, node_multipliers, node_speed_limits, node_device_limits)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	isReset := 0
	if pkg.IsReset {
		isReset = 1
	}

	trafficMode := pkg.TrafficMode
	if trafficMode == "" {
		trafficMode = "oneway"
	}

	result, err := r.db.ExecContext(ctx, query, name, pkg.Description, pkg.TrafficLimitBytes,
		pkg.CycleDays, isReset, pkg.ResetDay, string(nodesJSON), pkg.SpeedLimitMbps, pkg.DeviceLimit, autoSpeedJSON, shortCode, trafficMode, pkg.TemplateFilename, nodeMultJSON, nodeSpeedJSON, nodeDeviceJSON)
	if err != nil {
		return 0, fmt.Errorf("create package: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get last insert id: %w", err)
	}

	return id, nil
}

// 更新现有包模板
func (r *TrafficRepository) UpdatePackage(ctx context.Context, pkg Package) error {
	if r == nil || r.db == nil {
		return errors.New("traffic repository not initialized")
	}

	if pkg.ID <= 0 {
		return errors.New("package ID is required")
	}

	name := strings.TrimSpace(pkg.Name)
	if name == "" {
		return errors.New("package name is required")
	}

	// 将节点序列化为 JSON
	nodesJSON, err := json.Marshal(pkg.Nodes)
	if err != nil {
		return fmt.Errorf("serialize nodes: %w", err)
	}

	var autoSpeedJSON string
	if len(pkg.AutoSpeedRules) > 0 {
		b, _ := json.Marshal(pkg.AutoSpeedRules)
		autoSpeedJSON = string(b)
	}

	nodeMultJSON := serializeNodeMultipliers(pkg.NodeMultipliers, pkg.Nodes)
	nodeSpeedJSON := serializeNodeFloatMap(pkg.NodeSpeedLimits, pkg.Nodes)
	nodeDeviceJSON := serializeNodeIntMap(pkg.NodeDeviceLimits, pkg.Nodes)

	const query = `
		UPDATE packages
		SET name = ?, description = ?, traffic_limit_bytes = ?, cycle_days = ?,
		    is_reset = ?, reset_day = ?, nodes = ?, speed_limit_mbps = ?, device_limit = ?,
		    auto_speed_limit_json = ?, traffic_mode = ?, template_filename = ?, node_multipliers = ?,
		    node_speed_limits = ?, node_device_limits = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	isReset := 0
	if pkg.IsReset {
		isReset = 1
	}

	trafficMode := pkg.TrafficMode
	if trafficMode == "" {
		trafficMode = "oneway"
	}

	result, err := r.db.ExecContext(ctx, query, name, pkg.Description, pkg.TrafficLimitBytes,
		pkg.CycleDays, isReset, pkg.ResetDay, string(nodesJSON), pkg.SpeedLimitMbps, pkg.DeviceLimit, autoSpeedJSON, trafficMode, pkg.TemplateFilename, nodeMultJSON, nodeSpeedJSON, nodeDeviceJSON, pkg.ID)
	if err != nil {
		return fmt.Errorf("update package: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}

	if affected == 0 {
		return ErrPackageNotFound
	}

	return nil
}

// 根据 ID 删除包模板
func (r *TrafficRepository) DeletePackage(ctx context.Context, id int64) error {
	if r == nil || r.db == nil {
		return errors.New("traffic repository not initialized")
	}

	if id <= 0 {
		return errors.New("package ID is required")
	}

	const query = `DELETE FROM packages WHERE id = ?`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete package: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}

	if affected == 0 {
		return ErrPackageNotFound
	}

	return nil
}

// --- UserPackageAssignment ---

type UserPackageAssignment struct {
	ID                 int64      `json:"id"`
	Username           string     `json:"username"`
	PackageID          int64      `json:"package_id"`
	PackageStartDate   *time.Time `json:"package_start_date"`
	PackageEndDate     *time.Time `json:"package_end_date"`
	IsReset            bool       `json:"is_reset"`
	ResetDay           int        `json:"reset_day"`
	LastResetAt        *time.Time `json:"last_reset_at"`
	TrafficLimitOverride *int64   `json:"traffic_limit_override"`
	Status             string     `json:"status"` // active | expired | suspended
	IsPrimary          bool       `json:"is_primary"`
	ShortCode          string     `json:"short_code"`
	TrafficWarned80    bool       `json:"traffic_warned_80"`
	OverLimitEnforced  bool       `json:"over_limit_enforced"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

const userPackageAssignmentsSchema = `
CREATE TABLE IF NOT EXISTS user_package_assignments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL,
    package_id INTEGER NOT NULL,
    package_start_date TIMESTAMP,
    package_end_date TIMESTAMP,
    is_reset INTEGER NOT NULL DEFAULT 0,
    reset_day INTEGER NOT NULL DEFAULT 1,
    last_reset_at TIMESTAMP,
    traffic_limit_override INTEGER,
    status TEXT NOT NULL DEFAULT 'active',
    is_primary INTEGER NOT NULL DEFAULT 0,
    short_code TEXT NOT NULL UNIQUE,
    traffic_warned_80 INTEGER NOT NULL DEFAULT 0,
    over_limit_enforced INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(username) REFERENCES users(username) ON DELETE CASCADE,
    FOREIGN KEY(package_id) REFERENCES packages(id) ON DELETE CASCADE
);
`

func (r *TrafficRepository) migrateUserPackageAssignments() error {
	_, err := r.db.Exec(userPackageAssignmentsSchema)
	return err
}

// --- InviteCode ---

type InviteCode struct {
	Code           string     `json:"code"`
	Kind           string     `json:"kind"` // new | bind
	BindUsername   string     `json:"bind_username"`
	CreatedBy      string     `json:"created_by"`
	PackageID      *int64     `json:"package_id"`
	MaxUses        int        `json:"max_uses"`
	UsedCount      int        `json:"used_count"`
	ExpiresAt      *time.Time `json:"expires_at"`
	Revoked        bool       `json:"revoked"`
	Remark         string     `json:"remark"`
	CreatedAt      time.Time  `json:"created_at"`
	DurationMonths int        `json:"duration_months"`
}

const inviteCodesSchema = `
CREATE TABLE IF NOT EXISTS invite_codes (
    code           TEXT PRIMARY KEY,
    kind           TEXT NOT NULL CHECK (kind IN ('new', 'bind')),
    bind_username  TEXT NOT NULL DEFAULT '',
    created_by     TEXT NOT NULL,
    package_id     INTEGER,
    max_uses       INTEGER NOT NULL DEFAULT 1,
    used_count     INTEGER NOT NULL DEFAULT 0,
    expires_at     TIMESTAMP,
    revoked        INTEGER NOT NULL DEFAULT 0,
    remark         TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    duration_months INTEGER NOT NULL DEFAULT 0
);
`

func (r *TrafficRepository) migrateInviteCodes() error {
	_, err := r.db.Exec(inviteCodesSchema)
	return err
}

// --- InviteCodeUse ---

type InviteCodeUse struct {
	Code     string    `json:"code"`
	Username string    `json:"username"`
	TGID     *int64    `json:"tg_id"`
	UsedAt   time.Time `json:"used_at"`
}

const inviteCodeUsesSchema = `
CREATE TABLE IF NOT EXISTS invite_code_uses (
    code       TEXT NOT NULL,
    username   TEXT NOT NULL,
    tg_id      INTEGER,
    used_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (code, username)
);
`

func (r *TrafficRepository) migrateInviteCodeUses() error {
	_, err := r.db.Exec(inviteCodeUsesSchema)
	return err
}

// --- RenewalRequest ---

type RenewalRequest struct {
	ID             int64      `json:"id"`
	RequestToken   string     `json:"request_token"`
	Username       string     `json:"username"`
	TelegramID     int64      `json:"telegram_id"`
	PackageID      int64      `json:"package_id"`
	PackageName    string     `json:"package_name"`
	PreviousEndDate *time.Time `json:"previous_end_date"`
	RenewDays      int        `json:"renew_days"`
	Passphrase     string     `json:"-"`
	Source         string     `json:"source"` // web | telegram
	Status         string     `json:"status"` // pending | approved | rejected
	ReviewedBy     int64      `json:"reviewed_by"`
	ReviewedAt     *time.Time `json:"reviewed_at"`
	NewEndDate     *time.Time `json:"new_end_date"`
	ErrorMessage   string     `json:"error_message"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

const renewalRequestsSchema = `
CREATE TABLE IF NOT EXISTS renewal_requests (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    request_token TEXT NOT NULL UNIQUE,
    username TEXT NOT NULL,
    telegram_id INTEGER NOT NULL DEFAULT 0,
    package_id INTEGER NOT NULL,
    package_name TEXT NOT NULL DEFAULT '',
    previous_end_date TIMESTAMP,
    renew_days INTEGER NOT NULL,
    passphrase TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT 'web',
    status TEXT NOT NULL DEFAULT 'pending',
    reviewed_by INTEGER NOT NULL DEFAULT 0,
    reviewed_at TIMESTAMP,
    new_end_date TIMESTAMP,
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

func (r *TrafficRepository) migrateRenewalRequests() error {
	_, err := r.db.Exec(renewalRequestsSchema)
	return err
}

// --- UserTrafficRecord (per-user traffic totals per date) ---

type UserTrafficRecord struct {
	Username      string    `json:"username"`
	Date          string    `json:"date"`
	TotalLimit    int64     `json:"total_limit"`
	TotalUsed     int64     `json:"total_used"`
	TotalRemaining int64    `json:"total_remaining"`
	CreatedAt     time.Time `json:"created_at"`
}

const userTrafficRecordsSchema = `
CREATE TABLE IF NOT EXISTS user_traffic_records (
    username TEXT NOT NULL,
    date TEXT NOT NULL,
    total_limit INTEGER NOT NULL,
    total_used INTEGER NOT NULL,
    total_remaining INTEGER NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (username, date)
);
`

func (r *TrafficRepository) migrateUserTrafficRecords() error {
	_, err := r.db.Exec(userTrafficRecordsSchema)
	return err
}

// --- UserTrafficCycleCarry ---

type UserTrafficCycleCarry struct {
	Username         string    `json:"username"`
	WeightedUplink   float64   `json:"weighted_uplink"`
	WeightedDownlink float64   `json:"weighted_downlink"`
	UpdatedAt        time.Time `json:"updated_at"`
}

const userTrafficCycleCarrySchema = `
CREATE TABLE IF NOT EXISTS user_traffic_cycle_carry (
    username TEXT PRIMARY KEY,
    weighted_uplink REAL NOT NULL DEFAULT 0,
    weighted_downlink REAL NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

func (r *TrafficRepository) migrateUserTrafficCycleCarry() error {
	_, err := r.db.Exec(userTrafficCycleCarrySchema)
	return err
}

// --- UserRoutedOutboundAction ---

type UserRoutedOutboundAction struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Action    string    `json:"action"`
	CreatedAt time.Time `json:"created_at"`
}

const userRoutedOutboundActionsSchema = `
CREATE TABLE IF NOT EXISTS user_routed_outbound_actions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL,
    action TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

func (r *TrafficRepository) migrateUserRoutedOutboundActions() error {
	_, err := r.db.Exec(userRoutedOutboundActionsSchema)
	return err
}

// --- UserSubaccount ---

type UserSubaccount struct {
	ID              int64     `json:"id"`
	Username        string    `json:"username"`
	RoutedNodeID    int64     `json:"routed_node_id"`
	Email           string    `json:"email"`
	CredentialJSON  string    `json:"credential_json"`
	IsActive        bool      `json:"is_active"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

const userSubaccountsSchema = `
CREATE TABLE IF NOT EXISTS user_subaccounts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL,
    routed_node_id INTEGER NOT NULL,
    email TEXT NOT NULL,
    credential_json TEXT NOT NULL,
    is_active INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(routed_node_id, username),
    UNIQUE(routed_node_id, email),
    FOREIGN KEY(username) REFERENCES users(username) ON DELETE CASCADE
);
`

func (r *TrafficRepository) migrateUserSubaccounts() error {
	_, err := r.db.Exec(userSubaccountsSchema)
	return err
}

// --- UserAPIToken ---

type UserAPIToken struct {
	ID         int64      `json:"id"`
	Username   string     `json:"username"`
	Name       string     `json:"name"`
	TokenHash  string     `json:"-"`
	LastUsedAt *time.Time `json:"last_used_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

const userAPITokensSchema = `
CREATE TABLE IF NOT EXISTS user_api_tokens (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    username     TEXT NOT NULL,
    name         TEXT NOT NULL DEFAULT '',
    token_hash   TEXT NOT NULL UNIQUE,
    created_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at TIMESTAMP
);
`

func (r *TrafficRepository) migrateUserAPITokens() error {
	_, err := r.db.Exec(userAPITokensSchema)
	return err
}
