package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// RemoteServer represents a remote proxy server managed via agent token (push/pull model).
// Mirrors mmwX remote_servers table structure.
type RemoteServer struct {
	ID			int64		`json:"id"`
	Name			string		`json:"name"`
	Token			string		`json:"-"`	// never expose to frontend directly
	Status			string		`json:"status"`	// pending | connected | offline
	LastHeartbeat		*time.Time	`json:"last_heartbeat"`
	IPAddress		string		`json:"ip_address"`
	CreatedAt		time.Time	`json:"created_at"`
	UpdatedAt		time.Time	`json:"updated_at"`
	BootTime		*time.Time	`json:"boot_time"`
	XrayBootTime		*time.Time	`json:"xray_boot_time"`
	BootCount		int		`json:"boot_count"`
	XrayBootCount		int		`json:"xray_boot_count"`
	TokenExpiresAt		*time.Time	`json:"token_expires_at"`
	LastTokenRefresh	*time.Time	`json:"last_token_refresh"`
	ConnectionMode		string		`json:"connection_mode"`	// push | pull
	PullAddress		string		`json:"pull_address"`
	PullPort		int		`json:"pull_port"`
	PullToken		string		`json:"-"`
	LastPullAt		*time.Time	`json:"last_pull_at"`
	PushFailCount		int		`json:"push_fail_count"`
	LastPushFail		*time.Time	`json:"last_push_fail"`
	FallbackToPull		bool		`json:"fallback_to_pull"`
	FallbackAt		*time.Time	`json:"fallback_at"`
	CurrentUploadSpeed	int		`json:"current_upload_speed"`
	CurrentDownloadSpeed	int		`json:"current_download_speed"`
	SpeedUpdatedAt		*time.Time	`json:"speed_updated_at"`
	XrayRunning		bool		`json:"xray_running"`
	XrayVersion		string		`json:"xray_version"`
	XrayScannedAt		*time.Time	`json:"xray_scanned_at"`
	ListenPort		int		`json:"listen_port"`
	TrafficLimit		int64		`json:"traffic_limit"`
	TrafficResetDay		int		`json:"traffic_reset_day"`
	Domain			string		`json:"domain"`
	IPAddressV6		string		`json:"ip_address_v6"`
	WarpInstalled		bool		`json:"warp_installed"`
	AgentToken		string		`json:"-"`
	AgentTokenExpiresAt	*time.Time	`json:"agent_token_expires_at"`
	LastAgentTokenRefresh	*time.Time	`json:"last_agent_token_refresh"`
	Use443			bool		`json:"use_443"`
	StealMode		string		`json:"steal_mode"`
	SiteType		string		`json:"site_type"`
	SiteValue		string		`json:"site_value"`
	TimeOffsetSeconds	*int		`json:"time_offset_seconds"`
	XrayMode		string		`json:"xray_mode"`
	TrafficUsedOffset	int64		`json:"traffic_used_offset"`
	SortOrder		int		`json:"sort_order"`
	TrafficStatsMode	string		`json:"traffic_stats_mode"`
	TrafficSource		string		`json:"traffic_source"`
	SystemRxCycle		int64		`json:"system_rx_cycle"`
	SystemTxCycle		int64		`json:"system_tx_cycle"`
	SystemLastSeenRx	int64		`json:"system_last_seen_rx"`
	SystemLastSeenTx	int64		`json:"system_last_seen_tx"`
	SystemBootTimeUnix	int64		`json:"system_boot_time_unix"`
	SystemTrafficUpdatedAt	*time.Time	`json:"system_traffic_updated_at"`
	DDNSEnabled		bool		`json:"ddns_enabled"`
	DDNSProviderID		int		`json:"ddns_provider_id"`
	DDNSLastSyncedAt	*time.Time	`json:"ddns_last_synced_at"`
	DDNSLastError		string		`json:"ddns_last_error"`
	DDNSPending		bool		`json:"ddns_pending"`
	PullAddressV6		string		`json:"pull_address_v6"`
	DomainV6		string		`json:"domain_v6"`
	Ipv6Enabled		bool		`json:"ipv6_enabled"`
	ObservedIP		string		`json:"observed_ip"`
	ObservedIPV6		string		`json:"observed_ip_v6"`
	OfflineSince		*time.Time	`json:"offline_since"`
	OfflineNotified		bool		`json:"offline_notified"`
	SameHostAsMaster	bool		`json:"same_host_as_master"`
	IncludeInTrafficStats	bool		`json:"include_in_traffic_stats"`
	LockEntryIP		bool		`json:"lock_entry_ip"`
	PortRangeMin		int		`json:"port_range_min"`
	PortRangeMax		int		`json:"port_range_max"`
	LastTrafficResetAt	*time.Time	`json:"last_traffic_reset_at"`
	Region			string		`json:"region"`
	RegionCountry		string		`json:"region_country"`
	RegionName		string		`json:"region_name"`
	RegionCity		string		`json:"region_city"`
	RenewalPrice		float64		`json:"renewal_price"`
	RenewalCycle		string		`json:"renewal_cycle"`
	RenewalCurrency		string		`json:"renewal_currency"`
	ProviderName		string		`json:"provider_name"`
	ProviderURL		string		`json:"provider_url"`
	TelecomPaidPeer		bool		`json:"telecom_paid_peer"`
	ProviderUpdatedAt	*time.Time	`json:"provider_updated_at"`
	ExpiresAt		*time.Time	`json:"expires_at"`
	IsFederated		bool		`json:"is_federated"`        // 非持久化字段:是否为接入的"分享服务器"(联邦)
	FederationPrefix		string		`json:"federation_prefix"` // 非持久化字段:分享服务器上新增入站的 tag 前缀
}

// remoteServersSchema is the core schema. We start with essential columns and use
// ensureRemoteServerColumn for the many incremental additions (matching mmwX's evolution).
const remoteServersSchema = `
CREATE TABLE IF NOT EXISTS remote_servers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    token TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'connected', 'offline')),
    last_heartbeat TIMESTAMP,
    ip_address TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

func (r *TrafficRepository) migrateRemoteServers() error {
	if _, err := r.db.Exec(remoteServersSchema); err != nil {
		return fmt.Errorf("migrate remote_servers: %w", err)
	}
	// Incremental columns matching mmwX's 84-column table
	columns := []struct{ name, def string }{
		{"boot_time", "TIMESTAMP"},
		{"xray_boot_time", "TIMESTAMP"},
		{"boot_count", "INTEGER NOT NULL DEFAULT 0"},
		{"xray_boot_count", "INTEGER NOT NULL DEFAULT 0"},
		{"token_expires_at", "TIMESTAMP"},
		{"last_token_refresh", "TIMESTAMP"},
		{"connection_mode", "TEXT NOT NULL DEFAULT 'push'"},
		{"pull_address", "TEXT NOT NULL DEFAULT ''"},
		{"pull_port", "INTEGER NOT NULL DEFAULT 0"},
		{"pull_token", "TEXT NOT NULL DEFAULT ''"},
		{"last_pull_at", "TIMESTAMP"},
		{"push_fail_count", "INTEGER NOT NULL DEFAULT 0"},
		{"last_push_fail", "TIMESTAMP"},
		{"fallback_to_pull", "INTEGER NOT NULL DEFAULT 0"},
		{"fallback_at", "TIMESTAMP"},
		{"current_upload_speed", "INTEGER NOT NULL DEFAULT 0"},
		{"current_download_speed", "INTEGER NOT NULL DEFAULT 0"},
		{"speed_updated_at", "TIMESTAMP"},
		{"xray_running", "INTEGER NOT NULL DEFAULT 0"},
		{"xray_version", "TEXT NOT NULL DEFAULT ''"},
		{"xray_scanned_at", "TIMESTAMP"},
		{"listen_port", "INTEGER NOT NULL DEFAULT 0"},
		{"traffic_limit", "INTEGER NOT NULL DEFAULT 0"},
		{"traffic_reset_day", "INTEGER NOT NULL DEFAULT 0"},
		{"domain", "TEXT NOT NULL DEFAULT ''"},
		{"ip_address_v6", "TEXT NOT NULL DEFAULT ''"},
		{"warp_installed", "INTEGER NOT NULL DEFAULT 0"},
		{"agent_token", "TEXT NOT NULL DEFAULT ''"},
		{"agent_token_expires_at", "TIMESTAMP"},
		{"last_agent_token_refresh", "TIMESTAMP"},
		{"use_443", "INTEGER NOT NULL DEFAULT 0"},
		{"steal_mode", "TEXT NOT NULL DEFAULT 'tunnel'"},
		{"site_type", "TEXT NOT NULL DEFAULT ''"},
		{"site_value", "TEXT NOT NULL DEFAULT ''"},
		{"time_offset_seconds", "INTEGER"},
		{"xray_mode", "TEXT NOT NULL DEFAULT 'external'"},
		{"traffic_used_offset", "INTEGER NOT NULL DEFAULT 0"},
		{"sort_order", "INTEGER NOT NULL DEFAULT 0"},
		{"traffic_stats_mode", "TEXT NOT NULL DEFAULT 'both'"},
		{"traffic_source", "TEXT NOT NULL DEFAULT 'xray'"},
		{"system_rx_cycle", "INTEGER NOT NULL DEFAULT 0"},
		{"system_tx_cycle", "INTEGER NOT NULL DEFAULT 0"},
		{"system_last_seen_rx", "INTEGER NOT NULL DEFAULT 0"},
		{"system_last_seen_tx", "INTEGER NOT NULL DEFAULT 0"},
		{"system_boot_time_unix", "INTEGER NOT NULL DEFAULT 0"},
		{"system_traffic_updated_at", "TIMESTAMP"},
		{"ddns_enabled", "INTEGER NOT NULL DEFAULT 0"},
		{"ddns_provider_id", "INTEGER NOT NULL DEFAULT 0"},
		{"ddns_last_synced_at", "TIMESTAMP"},
		{"ddns_last_error", "TEXT NOT NULL DEFAULT ''"},
		{"ddns_pending", "INTEGER NOT NULL DEFAULT 0"},
		{"pull_address_v6", "TEXT NOT NULL DEFAULT ''"},
		{"domain_v6", "TEXT NOT NULL DEFAULT ''"},
		{"ipv6_enabled", "INTEGER NOT NULL DEFAULT 1"},
		{"observed_ip", "TEXT NOT NULL DEFAULT ''"},
		{"observed_ip_v6", "TEXT NOT NULL DEFAULT ''"},
		{"offline_since", "TIMESTAMP"},
		{"offline_notified", "INTEGER NOT NULL DEFAULT 0"},
		{"same_host_as_master", "INTEGER NOT NULL DEFAULT 0"},
		{"include_in_traffic_stats", "INTEGER NOT NULL DEFAULT 1"},
		{"lock_entry_ip", "INTEGER NOT NULL DEFAULT 0"},
		{"port_range_min", "INTEGER NOT NULL DEFAULT 0"},
		{"port_range_max", "INTEGER NOT NULL DEFAULT 0"},
		{"last_traffic_reset_at", "TIMESTAMP"},
		{"region", "TEXT NOT NULL DEFAULT ''"},
		{"region_country", "TEXT NOT NULL DEFAULT ''"},
		{"region_name", "TEXT NOT NULL DEFAULT ''"},
		{"region_city", "TEXT NOT NULL DEFAULT ''"},
		{"renewal_price", "NUMERIC NOT NULL DEFAULT 0"},
		{"renewal_cycle", "TEXT NOT NULL DEFAULT 'month'"},
		{"renewal_currency", "TEXT NOT NULL DEFAULT 'CNY'"},
		{"provider_name", "TEXT NOT NULL DEFAULT ''"},
		{"provider_url", "TEXT NOT NULL DEFAULT ''"},
		{"telecom_paid_peer", "INTEGER NOT NULL DEFAULT 0"},
		{"provider_updated_at", "TIMESTAMP"},
		{"expires_at", "TIMESTAMP"},
	}
	for _, col := range columns {
		if err := r.ensureRemoteServerColumn(col.name, col.def); err != nil {
			return err
		}
	}
	return nil
}

func (r *TrafficRepository) ensureRemoteServerColumn(name, definition string) error {
	rows, err := r.db.Query(`PRAGMA table_info(remote_servers)`)
	if err != nil {
		return fmt.Errorf("remote_servers table info: %w", err)
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
	alter := fmt.Sprintf("ALTER TABLE remote_servers ADD COLUMN %s %s", name, definition)
	if _, err := r.db.Exec(alter); err != nil {
		return fmt.Errorf("add column %s to remote_servers: %w", name, err)
	}
	return nil
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if toLowerByte(a[i]) != toLowerByte(b[i]) {
			return false
		}
	}
	return true
}

func toLowerByte(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + 32
	}
	return b
}

// --- ServerReturnRoute ---

type ServerReturnRoute struct {
	ServerID	int64		`json:"server_id"`
	Carrier		string		`json:"carrier"`
	Region		string		`json:"region"`
	RouteType	string		`json:"route_type"`
	EntryIP		string		`json:"entry_ip"`
	EntryASN	string		`json:"entry_asn"`
	Reason		string		`json:"reason"`
	TestedAt	time.Time	`json:"tested_at"`
}

const serverReturnRoutesSchema = `
CREATE TABLE IF NOT EXISTS server_return_routes (
    server_id  INTEGER NOT NULL,
    carrier    TEXT NOT NULL,
    region     TEXT NOT NULL DEFAULT '',
    route_type TEXT NOT NULL DEFAULT 'Unknown',
    entry_ip   TEXT NOT NULL DEFAULT '',
    entry_asn  TEXT NOT NULL DEFAULT '',
    reason     TEXT NOT NULL DEFAULT '',
    tested_at  TIMESTAMP NOT NULL,
    PRIMARY KEY (server_id, carrier)
);
`

func (r *TrafficRepository) migrateServerReturnRoutes() error {
	_, err := r.db.Exec(serverReturnRoutesSchema)
	return err
}

// --- ServerSystemTrafficSnapshot ---

type ServerSystemTrafficSnapshot struct {
	ID		int64		`json:"id"`
	ServerID	int64		`json:"server_id"`
	Date		string		`json:"date"`
	RxCycle		int64		`json:"rx_cycle"`
	TxCycle		int64		`json:"tx_cycle"`
	CreatedAt	time.Time	`json:"created_at"`
}

const serverSystemTrafficSnapshotsSchema = `
CREATE TABLE IF NOT EXISTS server_system_traffic_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id INTEGER NOT NULL,
    date TEXT NOT NULL,
    rx_cycle INTEGER NOT NULL DEFAULT 0,
    tx_cycle INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(server_id, date)
);
`

func (r *TrafficRepository) migrateServerSystemTrafficSnapshots() error {
	_, err := r.db.Exec(serverSystemTrafficSnapshotsSchema)
	return err
}

// --- ServerXrayConfigSnapshot ---

const serverXrayConfigSnapshotsSchema = `
CREATE TABLE IF NOT EXISTS server_xray_config_snapshots (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id   INTEGER NOT NULL,
    config_json TEXT    NOT NULL,
    config_hash TEXT    NOT NULL,
    source      TEXT    NOT NULL,
    status      TEXT    NOT NULL,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (server_id) REFERENCES remote_servers(id) ON DELETE CASCADE
);
`

func (r *TrafficRepository) migrateServerXrayConfigSnapshots() error {
	_, err := r.db.Exec(serverXrayConfigSnapshotsSchema)
	return err
}

// --- NodeReachability ---

type NodeReachability struct {
	NodeID			int64		`json:"node_id"`
	Reachable		bool		`json:"reachable"`
	ConsecutiveFail		int		`json:"consecutive_fail"`
	Since			time.Time	`json:"since"`
	AnnouncedBlocked	bool		`json:"announced_blocked"`
}

const nodeReachabilitySchema = `
CREATE TABLE IF NOT EXISTS node_reachability (
    node_id INTEGER PRIMARY KEY,
    reachable INTEGER NOT NULL DEFAULT 1,
    consecutive_fail INTEGER NOT NULL DEFAULT 0,
    since TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    announced_blocked INTEGER NOT NULL DEFAULT 0
);
`

func (r *TrafficRepository) migrateNodeReachability() error {
	_, err := r.db.Exec(nodeReachabilitySchema)
	return err
}

// --- Full-column select helpers ---

// remoteServerSelectAll is the SELECT clause covering all 84+ columns.
const remoteServerSelectAll = `SELECT
    id, name, status, COALESCE(last_heartbeat, NULL), COALESCE(ip_address,''),
    created_at, updated_at, COALESCE(boot_time, NULL), COALESCE(xray_boot_time, NULL),
    boot_count, xray_boot_count, COALESCE(token_expires_at, NULL), COALESCE(last_token_refresh, NULL),
    connection_mode, pull_address, pull_port, pull_token, COALESCE(last_pull_at, NULL),
    push_fail_count, COALESCE(last_push_fail, NULL), fallback_to_pull, COALESCE(fallback_at, NULL),
    current_upload_speed, current_download_speed, COALESCE(speed_updated_at, NULL),
    xray_running, xray_version, COALESCE(xray_scanned_at, NULL), listen_port,
    traffic_limit, traffic_reset_day, domain, ip_address_v6, warp_installed,
    agent_token, COALESCE(agent_token_expires_at, NULL), COALESCE(last_agent_token_refresh, NULL),
    use_443, steal_mode, site_type, site_value, COALESCE(time_offset_seconds, NULL),
    xray_mode, traffic_used_offset, sort_order, traffic_stats_mode, traffic_source,
    system_rx_cycle, system_tx_cycle, system_last_seen_rx, system_last_seen_tx,
    system_boot_time_unix, COALESCE(system_traffic_updated_at, NULL),
    ddns_enabled, ddns_provider_id, COALESCE(ddns_last_synced_at, NULL), ddns_last_error, ddns_pending,
    pull_address_v6, domain_v6, ipv6_enabled, observed_ip, observed_ip_v6,
    COALESCE(offline_since, NULL), offline_notified, same_host_as_master, include_in_traffic_stats,
    lock_entry_ip, port_range_min, port_range_max, COALESCE(last_traffic_reset_at, NULL),
    region, region_country, region_name, region_city,
    renewal_price, renewal_cycle, renewal_currency, provider_name, provider_url,
    telecom_paid_peer, COALESCE(provider_updated_at, NULL), COALESCE(expires_at, NULL)
FROM remote_servers`

// remoteServerScanFields returns pointers for scanning a full RemoteServer row.
func remoteServerScanFields(s *RemoteServer) []any {
	return []any{
		&s.ID, &s.Name, &s.Status, &s.LastHeartbeat, &s.IPAddress,
		&s.CreatedAt, &s.UpdatedAt, &s.BootTime, &s.XrayBootTime,
		&s.BootCount, &s.XrayBootCount, &s.TokenExpiresAt, &s.LastTokenRefresh,
		&s.ConnectionMode, &s.PullAddress, &s.PullPort, &s.PullToken, &s.LastPullAt,
		&s.PushFailCount, &s.LastPushFail, &s.FallbackToPull, &s.FallbackAt,
		&s.CurrentUploadSpeed, &s.CurrentDownloadSpeed, &s.SpeedUpdatedAt,
		&s.XrayRunning, &s.XrayVersion, &s.XrayScannedAt, &s.ListenPort,
		&s.TrafficLimit, &s.TrafficResetDay, &s.Domain, &s.IPAddressV6, &s.WarpInstalled,
		&s.AgentToken, &s.AgentTokenExpiresAt, &s.LastAgentTokenRefresh,
		&s.Use443, &s.StealMode, &s.SiteType, &s.SiteValue, &s.TimeOffsetSeconds,
		&s.XrayMode, &s.TrafficUsedOffset, &s.SortOrder, &s.TrafficStatsMode, &s.TrafficSource,
		&s.SystemRxCycle, &s.SystemTxCycle, &s.SystemLastSeenRx, &s.SystemLastSeenTx,
		&s.SystemBootTimeUnix, &s.SystemTrafficUpdatedAt,
		&s.DDNSEnabled, &s.DDNSProviderID, &s.DDNSLastSyncedAt, &s.DDNSLastError, &s.DDNSPending,
		&s.PullAddressV6, &s.DomainV6, &s.Ipv6Enabled, &s.ObservedIP, &s.ObservedIPV6,
		&s.OfflineSince, &s.OfflineNotified, &s.SameHostAsMaster, &s.IncludeInTrafficStats,
		&s.LockEntryIP, &s.PortRangeMin, &s.PortRangeMax, &s.LastTrafficResetAt,
		&s.Region, &s.RegionCountry, &s.RegionName, &s.RegionCity,
		&s.RenewalPrice, &s.RenewalCycle, &s.RenewalCurrency, &s.ProviderName, &s.ProviderURL,
		&s.TelecomPaidPeer, &s.ProviderUpdatedAt, &s.ExpiresAt,
	}
}

func scanRemoteServer(rows *sql.Rows) (RemoteServer, error) {
	var s RemoteServer
	err := rows.Scan(remoteServerScanFields(&s)...)
	return s, err
}

// UpdateRemoteServerFields selectively updates server fields (mmwX-compatible).
// Only non-empty/non-zero fields are updated; omitted fields preserve existing values.
func (r *TrafficRepository) UpdateRemoteServerFields(ctx context.Context, id int64, updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}
	setParts := []string{}
	args := []any{}
	for k, v := range updates {
		setParts = append(setParts, fmt.Sprintf("%s = ?", k))
		args = append(args, v)
	}
	setParts = append(setParts, "updated_at = CURRENT_TIMESTAMP")
	args = append(args, id)
	query := fmt.Sprintf("UPDATE remote_servers SET %s WHERE id = ?", strings.Join(setParts, ", "))
	_, err := r.db.ExecContext(ctx, query, args...)
	return err
}

// UpdateRemoteServerRegion updates the region/geo fields for a server.
func (r *TrafficRepository) UpdateRemoteServerRegion(ctx context.Context, id int64, region, country, regionName, city string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE remote_servers SET region = ?, region_country = ?, region_name = ?, region_city = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		region, country, regionName, city, id)
	return err
}

// UpdateRemoteServerDDNS updates DDNS-related fields.
func (r *TrafficRepository) UpdateRemoteServerDDNS(ctx context.Context, id int64, enabled bool, providerID int, lastSyncedAt *time.Time, lastError string, pending bool) error {
	_, err := r.db.ExecContext(ctx, `UPDATE remote_servers SET ddns_enabled = ?, ddns_provider_id = ?, ddns_last_synced_at = ?, ddns_last_error = ?, ddns_pending = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		enabled, providerID, lastSyncedAt, lastError, pending, id)
	return err
}

// UpdateRemoteServerIPv6 updates IPv6-related fields.
func (r *TrafficRepository) UpdateRemoteServerIPv6(ctx context.Context, id int64, enabled bool, address, domain, pullAddress string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE remote_servers SET ipv6_enabled = ?, ip_address_v6 = ?, domain_v6 = ?, pull_address_v6 = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		enabled, address, domain, pullAddress, id)
	return err
}

// --- CRUD for RemoteServer ---

var ErrRemoteServerNotFound = errors.New("remote server not found")

func (r *TrafficRepository) CreateRemoteServer(ctx context.Context, s *RemoteServer) (int64, error) {
	res, err := r.db.ExecContext(ctx, `INSERT INTO remote_servers
		(name, token, status, ip_address, domain, connection_mode, pull_address, pull_port,
		 listen_port, xray_mode, steal_mode, traffic_limit, traffic_reset_day)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.Name, s.Token, s.Status, s.IPAddress, s.Domain, s.ConnectionMode, s.PullAddress, s.PullPort,
		s.ListenPort, s.XrayMode, s.StealMode, s.TrafficLimit, s.TrafficResetDay)
	if err != nil {
		return 0, fmt.Errorf("create remote server: %w", err)
	}
	return res.LastInsertId()
}

func (r *TrafficRepository) ListRemoteServers(ctx context.Context) ([]RemoteServer, error) {
	rows, err := r.db.QueryContext(ctx, remoteServerSelectAll+` ORDER BY sort_order ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list remote servers: %w", err)
	}
	defer rows.Close()
	var out []RemoteServer
	for rows.Next() {
		s, err := scanRemoteServer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *TrafficRepository) GetRemoteServer(ctx context.Context, id int64) (*RemoteServer, error) {
	var s RemoteServer
	err := r.db.QueryRowContext(ctx, remoteServerSelectAll+` WHERE id = ?`, id).Scan(remoteServerScanFields(&s)...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRemoteServerNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *TrafficRepository) GetRemoteServerByToken(ctx context.Context, token string) (*RemoteServer, error) {
	var s RemoteServer
	err := r.db.QueryRowContext(ctx, `SELECT id, name, token, status, COALESCE(ip_address,''), created_at, updated_at FROM remote_servers WHERE token = ?`, token).
		Scan(&s.ID, &s.Name, &s.Token, &s.Status, &s.IPAddress, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRemoteServerNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *TrafficRepository) UpdateRemoteServerStatus(ctx context.Context, id int64, status string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE remote_servers SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, status, id)
	return err
}

func (r *TrafficRepository) UpdateRemoteServerHeartbeat(ctx context.Context, token string, ipAddress string, ipAddressV6 string) (bool, *RemoteServer, error) {
	if r == nil || r.db == nil {
		return false, nil, errors.New("traffic repository not initialized")
	}

	token = strings.TrimSpace(token)
	if token == "" {
		return false, nil, errors.New("remote server token is required")
	}

	server, err := r.GetRemoteServerByToken(ctx, token)
	if err != nil {
		return false, nil, err
	}

	const stmt = `UPDATE remote_servers SET
		status = ?,
		last_heartbeat = CURRENT_TIMESTAMP,
		ip_address = COALESCE(NULLIF(?, ''), ip_address),
		ip_address_v6 = COALESCE(NULLIF(?, ''), ip_address_v6),
		offline_since = NULL,
		offline_notified = 0,
		updated_at = CURRENT_TIMESTAMP
		WHERE token = ?`

	res, err := r.db.ExecContext(ctx, stmt, RemoteServerStatusConnected, ipAddress, ipAddressV6, token)
	if err != nil {
		return false, nil, fmt.Errorf("update remote server heartbeat: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return false, nil, fmt.Errorf("get rows affected: %w", err)
	}

	if affected == 0 {
		return false, nil, ErrRemoteServerNotFound
	}

	v4Changed := ipAddress != "" && ipAddress != server.IPAddress
	v6Changed := ipAddressV6 != "" && ipAddressV6 != server.IPAddressV6
	if v4Changed || v6Changed {
		latest := *server
		if v4Changed {
			latest.IPAddress = ipAddress
		}
		if v6Changed {
			latest.IPAddressV6 = ipAddressV6
		}
		return true, &latest, nil
	}
	return false, nil, nil
}

func (r *TrafficRepository) DeleteRemoteServer(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM remote_servers WHERE id = ?`, id)
	return err
}
