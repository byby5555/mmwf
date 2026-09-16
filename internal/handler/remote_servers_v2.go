package handler

// remote_servers.go — Remote server management API (agent push/pull model).
//
// This handler covers the mmwX remote_servers table with all 84 columns:
// DDNS, region/geo, IPv6, renewal, port range, traffic stats mode, etc.
//
// Routes: /api/admin/remote-servers, /api/admin/remote-servers/{id}/{action}

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"miaomiaowux/internal/logger"
	"miaomiaowux/internal/storage"
)

type remoteServersHandler struct {
	repo *storage.TrafficRepository
}

func NewRemoteServersHandler(repo *storage.TrafficRepository) http.Handler {
	return &remoteServersHandler{repo: repo}
}

// remoteServerResponse is the full response returned to frontend.
// Sensitive fields (token, pull_token, agent_token) are never included.
type remoteServerResponse struct {
	// Basic
	ID            int64      `json:"id"`
	Name          string     `json:"name"`
	Status        string     `json:"status"`
	LastHeartbeat *time.Time `json:"last_heartbeat"`
	IPAddress     string     `json:"ip_address"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`

	// Connection
	ConnectionMode string `json:"connection_mode"` // push | pull | auto
	PullAddress    string `json:"pull_address"`
	PullPort       int    `json:"pull_port"`
	HasPullToken   bool   `json:"has_pull_token"`
	LastPullAt     *time.Time `json:"last_pull_at"`
	PushFailCount  int    `json:"push_fail_count"`
	LastPushFail   *time.Time `json:"last_push_fail"`
	FallbackToPull bool   `json:"fallback_to_pull"`
	FallbackAt     *time.Time `json:"fallback_at"`

	// Xray/sing-box
	XrayRunning   bool       `json:"xray_running"`
	XrayVersion   string     `json:"xray_version"`
	XrayScannedAt *time.Time `json:"xray_scanned_at"`
	ListenPort    int        `json:"listen_port"`
	XrayMode      string     `json:"xray_mode"` // embedded | external
	Use443        bool       `json:"use_443"`
	StealMode     string     `json:"steal_mode"` // tunnel | fallback | self
	SiteType      string     `json:"site_type"`
	SiteValue     string     `json:"site_value"`

	// Traffic
	TrafficLimit        int64      `json:"traffic_limit"`
	TrafficResetDay     int        `json:"traffic_reset_day"`
	TrafficUsedOffset   int64      `json:"traffic_used_offset"`
	LastTrafficResetAt  *time.Time `json:"last_traffic_reset_at"`
	TrafficStatsMode    string     `json:"traffic_stats_mode"` // both | upload | download | max
	TrafficSource       string     `json:"traffic_source"`      // xray | system
	CurrentUploadSpeed  int        `json:"current_upload_speed"`
	CurrentDownloadSpeed int       `json:"current_download_speed"`
	SpeedUpdatedAt      *time.Time `json:"speed_updated_at"`

	// System NIC traffic
	SystemRxCycle         int64      `json:"system_rx_cycle"`
	SystemTxCycle         int64      `json:"system_tx_cycle"`
	SystemLastSeenRx      int64      `json:"system_last_seen_rx"`
	SystemLastSeenTx      int64      `json:"system_last_seen_tx"`
	SystemBootTimeUnix    int64      `json:"system_boot_time_unix"`
	SystemTrafficUpdatedAt *time.Time `json:"system_traffic_updated_at"`

	// Domain / IPv6
	Domain      string `json:"domain"`
	DomainV6    string `json:"domain_v6"`
	IPAddressV6 string `json:"ip_address_v6"`
	Ipv6Enabled bool   `json:"ipv6_enabled"`
	PullAddressV6 string `json:"pull_address_v6"`

	// Region / Geo
	Region       string `json:"region"`        // emoji flag
	RegionCountry string `json:"region_country"` // ISO code
	RegionName   string `json:"region_name"`
	RegionCity   string `json:"region_city"`

	// DDNS
	DDNSEnabled      bool       `json:"ddns_enabled"`
	DDNSProviderID  int        `json:"ddns_provider_id"`
	DDNSLastSyncedAt *time.Time `json:"ddns_last_synced_at"`
	DDNSLastError   string     `json:"ddns_last_error"`
	DDNSPending     bool       `json:"ddns_pending"`

	// Renewal / Provider
	RenewalPrice    float64    `json:"renewal_price"`
	RenewalCycle    string     `json:"renewal_cycle"`
	RenewalCurrency string     `json:"renewal_currency"`
	ProviderName    string     `json:"provider_name"`
	ProviderURL     string     `json:"provider_url"`
	TelecomPaidPeer bool       `json:"telecom_paid_peer"`
	ProviderUpdatedAt *time.Time `json:"provider_updated_at"`
	ExpiresAt       *time.Time `json:"expires_at"`

	// Security / Access
	LockEntryIP         bool       `json:"lock_entry_ip"`
	ObservedIP          string     `json:"observed_ip"`
	ObservedIPV6        string     `json:"observed_ip_v6"`
	OfflineSince        *time.Time `json:"offline_since"`
	OfflineNotified     bool       `json:"offline_notified"`
	SameHostAsMaster    bool       `json:"same_host_as_master"`
	IncludeInTrafficStats bool     `json:"include_in_traffic_stats"`
	PortRangeMin        int        `json:"port_range_min"`
	PortRangeMax        int        `json:"port_range_max"`

	// Boot info
	BootTime      *time.Time `json:"boot_time"`
	XrayBootTime  *time.Time `json:"xray_boot_time"`
	BootCount     int        `json:"boot_count"`
	XrayBootCount int        `json:"xray_boot_count"`

	// Token info (existence only, not the actual token)
	TokenExpiresAt       *time.Time `json:"token_expires_at"`
	LastTokenRefresh     *time.Time `json:"last_token_refresh"`
	HasAgentToken        bool       `json:"has_agent_token"`
	AgentTokenExpiresAt  *time.Time `json:"agent_token_expires_at"`
	LastAgentTokenRefresh *time.Time `json:"last_agent_token_refresh"`

	// Misc
	TimeOffsetSeconds *int  `json:"time_offset_seconds"`
	SortOrder         int   `json:"sort_order"`
	WarpInstalled     bool  `json:"warp_installed"`
}

func toRemoteServerResponse(s storage.RemoteServer) remoteServerResponse {
	return remoteServerResponse{
		ID: s.ID, Name: s.Name, Status: s.Status,
		LastHeartbeat: s.LastHeartbeat, IPAddress: s.IPAddress,
		CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
		ConnectionMode: s.ConnectionMode, PullAddress: s.PullAddress, PullPort: s.PullPort,
		HasPullToken: s.PullToken != "", LastPullAt: s.LastPullAt,
		PushFailCount: s.PushFailCount, LastPushFail: s.LastPushFail,
		FallbackToPull: s.FallbackToPull, FallbackAt: s.FallbackAt,
		XrayRunning: s.XrayRunning, XrayVersion: s.XrayVersion, XrayScannedAt: s.XrayScannedAt,
		ListenPort: s.ListenPort, XrayMode: s.XrayMode, Use443: s.Use443,
		StealMode: s.StealMode, SiteType: s.SiteType, SiteValue: s.SiteValue,
		TrafficLimit: s.TrafficLimit, TrafficResetDay: s.TrafficResetDay,
		TrafficUsedOffset: s.TrafficUsedOffset, LastTrafficResetAt: s.LastTrafficResetAt,
		TrafficStatsMode: s.TrafficStatsMode, TrafficSource: s.TrafficSource,
		CurrentUploadSpeed: s.CurrentUploadSpeed, CurrentDownloadSpeed: s.CurrentDownloadSpeed,
		SpeedUpdatedAt: s.SpeedUpdatedAt,
		SystemRxCycle: s.SystemRxCycle, SystemTxCycle: s.SystemTxCycle,
		SystemLastSeenRx: s.SystemLastSeenRx, SystemLastSeenTx: s.SystemLastSeenTx,
		SystemBootTimeUnix: s.SystemBootTimeUnix, SystemTrafficUpdatedAt: s.SystemTrafficUpdatedAt,
		Domain: s.Domain, DomainV6: s.DomainV6, IPAddressV6: s.IPAddressV6,
		Ipv6Enabled: s.Ipv6Enabled, PullAddressV6: s.PullAddressV6,
		Region: s.Region, RegionCountry: s.RegionCountry, RegionName: s.RegionName, RegionCity: s.RegionCity,
		DDNSEnabled: s.DDNSEnabled, DDNSProviderID: s.DDNSProviderID,
		DDNSLastSyncedAt: s.DDNSLastSyncedAt, DDNSLastError: s.DDNSLastError, DDNSPending: s.DDNSPending,
		RenewalPrice: s.RenewalPrice, RenewalCycle: s.RenewalCycle, RenewalCurrency: s.RenewalCurrency,
		ProviderName: s.ProviderName, ProviderURL: s.ProviderURL, TelecomPaidPeer: s.TelecomPaidPeer,
		ProviderUpdatedAt: s.ProviderUpdatedAt, ExpiresAt: s.ExpiresAt,
		LockEntryIP: s.LockEntryIP, ObservedIP: s.ObservedIP, ObservedIPV6: s.ObservedIPV6,
		OfflineSince: s.OfflineSince, OfflineNotified: s.OfflineNotified,
		SameHostAsMaster: s.SameHostAsMaster, IncludeInTrafficStats: s.IncludeInTrafficStats,
		PortRangeMin: s.PortRangeMin, PortRangeMax: s.PortRangeMax,
		BootTime: s.BootTime, XrayBootTime: s.XrayBootTime, BootCount: s.BootCount, XrayBootCount: s.XrayBootCount,
		TokenExpiresAt: s.TokenExpiresAt, LastTokenRefresh: s.LastTokenRefresh,
		HasAgentToken: s.AgentToken != "", AgentTokenExpiresAt: s.AgentTokenExpiresAt,
		LastAgentTokenRefresh: s.LastAgentTokenRefresh,
		TimeOffsetSeconds: s.TimeOffsetSeconds, SortOrder: s.SortOrder, WarpInstalled: s.WarpInstalled,
	}
}

func (h *remoteServersHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.repo == nil {
		writeError(w, http.StatusInternalServerError, errors.New("repository not configured"))
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/admin/remote-servers")
	path = strings.Trim(path, "/")
	segments := strings.Split(path, "/")

	switch {
	case len(segments) == 1 && segments[0] == "":
		switch r.Method {
		case http.MethodGet:
			h.handleList(w, r)
		case http.MethodPost:
			h.handleCreate(w, r)
		default:
			methodNotAllowed(w, http.MethodGet, http.MethodPost)
		}
	case len(segments) >= 1 && segments[0] != "":
		id, err := strconv.ParseInt(segments[0], 10, 64)
		if err != nil {
			writeBadRequest(w, "无效的服务器 ID")
			return
		}
		action := ""
		if len(segments) >= 2 {
			action = segments[1]
		}
		h.handleItem(w, r, id, action)
	default:
		http.NotFound(w, r)
	}
}

func (h *remoteServersHandler) handleList(w http.ResponseWriter, r *http.Request) {
	servers, err := h.repo.ListRemoteServers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	resp := make([]remoteServerResponse, 0, len(servers))
	for _, s := range servers {
		resp = append(resp, toRemoteServerResponse(s))
	}
	respondJSON(w, http.StatusOK, resp)
}

// remoteServerCreateRequest is the payload for creating a new remote server.
type remoteServerCreateRequest struct {
	Name           string `json:"name"`
	IPAddress      string `json:"ip_address"`
	Domain         string `json:"domain"`
	ConnectionMode string `json:"connection_mode"` // push | pull | auto
	PullAddress    string `json:"pull_address"`
	PullPort       int    `json:"pull_port"`
	ListenPort     int    `json:"listen_port"`
	XrayMode       string `json:"xray_mode"` // embedded | external
	StealMode      string `json:"steal_mode"` // tunnel | fallback | self
	TrafficLimit   int64  `json:"traffic_limit"`
	TrafficResetDay int   `json:"traffic_reset_day"`
}

func (h *remoteServersHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req remoteServerCreateRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeBadRequest(w, "无效的请求体: "+err.Error())
		return
	}
	if req.Name == "" {
		writeBadRequest(w, "服务器名称不能为空")
		return
	}

	// Generate a random token for the server
	token, err := generateRemoteServerToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	srv := storage.RemoteServer{
		Name:           req.Name,
		Token:          token,
		Status:         "pending",
		IPAddress:      req.IPAddress,
		Domain:         req.Domain,
		ConnectionMode: req.ConnectionMode,
		PullAddress:   req.PullAddress,
		PullPort:       req.PullPort,
		ListenPort:     req.ListenPort,
		XrayMode:       req.XrayMode,
		StealMode:      req.StealMode,
		TrafficLimit:   req.TrafficLimit,
		TrafficResetDay: req.TrafficResetDay,
	}
	if srv.ConnectionMode == "" {
		srv.ConnectionMode = "auto"
	}
	if srv.XrayMode == "" {
		srv.XrayMode = "embedded"
	}
	if srv.StealMode == "" {
		srv.StealMode = "fallback"
	}

	id, err := h.repo.CreateRemoteServer(r.Context(), &srv)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	srv.ID = id
	respondJSON(w, http.StatusCreated, toRemoteServerResponse(srv))
}

func (h *remoteServersHandler) handleItem(w http.ResponseWriter, r *http.Request, id int64, action string) {
	switch {
	case action == "" && r.Method == http.MethodGet:
		h.handleGet(w, r, id)
	case action == "" && r.Method == http.MethodPut:
		h.handleUpdate(w, r, id)
	case action == "" && r.Method == http.MethodDelete:
		h.handleDelete(w, r, id)
	case action == "detect-region" && r.Method == http.MethodPost:
		h.handleDetectRegion(w, r, id)
	case action == "ddns-sync" && r.Method == http.MethodPost:
		h.handleDDNSSync(w, r, id)
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPost)
	}
}

func (h *remoteServersHandler) handleGet(w http.ResponseWriter, r *http.Request, id int64) {
	srv, err := h.repo.GetRemoteServer(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrRemoteServerNotFound) {
			http.NotFound(w, r)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	respondJSON(w, http.StatusOK, toRemoteServerResponse(*srv))
}

// remoteServerUpdateRequest supports partial updates to any field.
type remoteServerUpdateRequest struct {
	Name             *string  `json:"name"`
	Domain           *string  `json:"domain"`
	DomainV6         *string  `json:"domain_v6"`
	IPAddressV6      *string  `json:"ip_address_v6"`
	Ipv6Enabled      *bool    `json:"ipv6_enabled"`
	DDNSEnabled      *bool    `json:"ddns_enabled"`
	DDNSProviderID   *int     `json:"ddns_provider_id"`
	Region           *string  `json:"region"`
	RegionCountry    *string  `json:"region_country"`
	RegionName       *string  `json:"region_name"`
	RegionCity       *string  `json:"region_city"`
	RenewalPrice     *float64 `json:"renewal_price"`
	RenewalCycle     *string  `json:"renewal_cycle"`
	RenewalCurrency  *string  `json:"renewal_currency"`
	ProviderName     *string  `json:"provider_name"`
	ProviderURL      *string  `json:"provider_url"`
	TelecomPaidPeer  *bool    `json:"telecom_paid_peer"`
	ExpiresAt        *string  `json:"expires_at"` // ISO 8601
	LockEntryIP      *bool    `json:"lock_entry_ip"`
	PortRangeMin     *int     `json:"port_range_min"`
	PortRangeMax     *int     `json:"port_range_max"`
	TrafficLimit     *int64   `json:"traffic_limit"`
	TrafficResetDay  *int     `json:"traffic_reset_day"`
	TrafficStatsMode *string  `json:"traffic_stats_mode"`
	TrafficSource    *string  `json:"traffic_source"`
	IncludeInTrafficStats *bool `json:"include_in_traffic_stats"`
	StealMode        *string  `json:"steal_mode"`
	SiteType         *string  `json:"site_type"`
	SiteValue        *string  `json:"site_value"`
	XrayMode         *string  `json:"xray_mode"`
	Use443           *bool    `json:"use_443"`
	SortOrder        *int     `json:"sort_order"`
}

func (h *remoteServersHandler) handleUpdate(w http.ResponseWriter, r *http.Request, id int64) {
	var req remoteServerUpdateRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeBadRequest(w, "无效的请求体: "+err.Error())
		return
	}

	// Verify server exists
	if _, err := h.repo.GetRemoteServer(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrRemoteServerNotFound) {
			http.NotFound(w, r)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	updates := map[string]any{}
	if req.Name != nil { updates["name"] = *req.Name }
	if req.Domain != nil { updates["domain"] = *req.Domain }
	if req.DomainV6 != nil { updates["domain_v6"] = *req.DomainV6 }
	if req.IPAddressV6 != nil { updates["ip_address_v6"] = *req.IPAddressV6 }
	if req.Ipv6Enabled != nil { updates["ipv6_enabled"] = *req.Ipv6Enabled }
	if req.DDNSEnabled != nil { updates["ddns_enabled"] = *req.DDNSEnabled }
	if req.DDNSProviderID != nil { updates["ddns_provider_id"] = *req.DDNSProviderID }
	if req.Region != nil { updates["region"] = *req.Region }
	if req.RegionCountry != nil { updates["region_country"] = *req.RegionCountry }
	if req.RegionName != nil { updates["region_name"] = *req.RegionName }
	if req.RegionCity != nil { updates["region_city"] = *req.RegionCity }
	if req.RenewalPrice != nil { updates["renewal_price"] = *req.RenewalPrice }
	if req.RenewalCycle != nil { updates["renewal_cycle"] = *req.RenewalCycle }
	if req.RenewalCurrency != nil { updates["renewal_currency"] = *req.RenewalCurrency }
	if req.ProviderName != nil { updates["provider_name"] = *req.ProviderName }
	if req.ProviderURL != nil { updates["provider_url"] = *req.ProviderURL }
	if req.TelecomPaidPeer != nil { updates["telecom_paid_peer"] = *req.TelecomPaidPeer }
	if req.ExpiresAt != nil {
		if *req.ExpiresAt == "" {
			updates["expires_at"] = nil
		} else {
			if t, err := time.Parse(time.RFC3339, *req.ExpiresAt); err == nil {
				updates["expires_at"] = t
			}
		}
	}
	if req.LockEntryIP != nil { updates["lock_entry_ip"] = *req.LockEntryIP }
	if req.PortRangeMin != nil { updates["port_range_min"] = *req.PortRangeMin }
	if req.PortRangeMax != nil { updates["port_range_max"] = *req.PortRangeMax }
	if req.TrafficLimit != nil { updates["traffic_limit"] = *req.TrafficLimit }
	if req.TrafficResetDay != nil { updates["traffic_reset_day"] = *req.TrafficResetDay }
	if req.TrafficStatsMode != nil { updates["traffic_stats_mode"] = *req.TrafficStatsMode }
	if req.TrafficSource != nil { updates["traffic_source"] = *req.TrafficSource }
	if req.IncludeInTrafficStats != nil { updates["include_in_traffic_stats"] = *req.IncludeInTrafficStats }
	if req.StealMode != nil { updates["steal_mode"] = *req.StealMode }
	if req.SiteType != nil { updates["site_type"] = *req.SiteType }
	if req.SiteValue != nil { updates["site_value"] = *req.SiteValue }
	if req.XrayMode != nil { updates["xray_mode"] = *req.XrayMode }
	if req.Use443 != nil { updates["use_443"] = *req.Use443 }
	if req.SortOrder != nil { updates["sort_order"] = *req.SortOrder }

	if err := h.repo.UpdateRemoteServerFields(r.Context(), id, updates); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	// Return updated server
	srv, err := h.repo.GetRemoteServer(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	respondJSON(w, http.StatusOK, toRemoteServerResponse(*srv))
}

func (h *remoteServersHandler) handleDelete(w http.ResponseWriter, r *http.Request, id int64) {
	if err := h.repo.DeleteRemoteServer(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrRemoteServerNotFound) {
			http.NotFound(w, r)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

// handleDetectRegion auto-detects the geographic region of a server by its IP address.
// Uses a public IP geolocation API (ip-api.com) and updates the region fields.
func (h *remoteServersHandler) handleDetectRegion(w http.ResponseWriter, r *http.Request, id int64) {
	srv, err := h.repo.GetRemoteServer(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrRemoteServerNotFound) {
			http.NotFound(w, r)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	ip := srv.IPAddress
	if ip == "" {
		ip = srv.ObservedIP
	}
	if ip == "" {
		writeBadRequest(w, "服务器没有 IP 地址，无法识别区域")
		return
	}

	// Use ip-api.com (free, 45 req/min limit, returns JSON)
	geo, err := detectRegionByIP(r.Context(), ip)
	if err != nil {
		logger.Warn("[RemoteServers] 区域识别失败", "server_id", id, "ip", ip, "error", err)
		writeError(w, http.StatusBadGateway, err)
		return
	}

	// Update region fields
	emojiFlag := countryToEmojiFlag(geo.CountryCode)
	if err := h.repo.UpdateRemoteServerRegion(r.Context(), id, emojiFlag, geo.CountryCode, geo.Region, geo.City); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	logger.Info("[RemoteServers] 区域识别成功", "server_id", id, "ip", ip, "country", geo.CountryCode, "region", geo.Region, "city", geo.City)
	respondJSON(w, http.StatusOK, map[string]string{
		"region":        emojiFlag,
		"region_country": geo.CountryCode,
		"region_name":   geo.Region,
		"region_city":   geo.City,
	})
}

// handleDDNSSync triggers a DDNS sync for the server (Cloudflare API).
func (h *remoteServersHandler) handleDDNSSync(w http.ResponseWriter, r *http.Request, id int64) {
	srv, err := h.repo.GetRemoteServer(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrRemoteServerNotFound) {
			http.NotFound(w, r)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if !srv.DDNSEnabled {
		writeBadRequest(w, "该服务器未启用 DDNS")
		return
	}

	if srv.Domain == "" {
		writeBadRequest(w, "该服务器未配置域名")
		return
	}

	// TODO: Implement Cloudflare API sync
	// For now, just mark as pending
	if err := h.repo.UpdateRemoteServerDDNS(r.Context(), id, true, srv.DDNSProviderID, &time.Time{}, "DDNS sync not yet implemented", true); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "pending", "message": "DDNS sync queued"})
}

// generateRemoteServerToken generates a random token for a new remote server.
func generateRemoteServerToken() (string, error) {
	return generateRandomToken(32)
}

// --- IP Geolocation ---

type ipGeoInfo struct {
	CountryCode string `json:"countryCode"`
	Region      string `json:"regionName"`
	City        string `json:"city"`
}

func detectRegionByIP(ctx context.Context, ip string) (*ipGeoInfo, error) {
	// Use ip-api.com (free, no key required, 45 req/min)
	url := "http://ip-api.com/json/" + ip + "?fields=countryCode,regionName,city"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("geo API returned " + resp.Status)
	}
	var geo ipGeoInfo
	if err := json.NewDecoder(resp.Body).Decode(&geo); err != nil {
		return nil, err
	}
	if geo.CountryCode == "" {
		return nil, errors.New("empty geo result")
	}
	return &geo, nil
}

// countryToEmojiFlag converts a 2-letter country code to emoji flag.
func countryToEmojiFlag(code string) string {
	if len(code) != 2 {
		return ""
	}
	// Regional indicator symbols: each letter maps to 0x1F1E6 + (letter - 'A')
	r := []rune{}
	for _, c := range strings.ToUpper(code) {
		if c >= 'A' && c <= 'Z' {
			r = append(r, 0x1F1E6+rune(c-'A'))
		}
	}
	return string(r)
}

// generateRandomToken generates a cryptographically random base64url token.
func generateRandomToken(length int) (string, error) {
	buf := make([]byte, length)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
