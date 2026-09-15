package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"miaomiaowux/internal/auth"
	"miaomiaowux/internal/logger"
	"miaomiaowux/internal/singboxops"
	"miaomiaowux/internal/storage"
)

// serversHandler manages sing-box servers (xray_servers table, mmwX-compatible naming).
type serversHandler struct {
	repo *storage.TrafficRepository
}

func NewServersHandler(repo *storage.TrafficRepository) http.Handler {
	return &serversHandler{repo: repo}
}

// serverRequest is the create/update payload from the frontend.
type serverRequest struct {
	Name        string `json:"name"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	SSHUser     string `json:"ssh_user"`
	AuthType    string `json:"auth_type"`
	AuthData    string `json:"auth_data"` // empty on update = unchanged
	Description string `json:"description"`
	ConfigPath  string `json:"config_path"`
	APIPort     int    `json:"api_port"`
	SingboxPort int    `json:"singbox_port"`
	SingboxUser string `json:"singbox_user"`
	Enabled     *bool  `json:"enabled"`
	IsLocal     *bool  `json:"is_local"`
	IsPrimary   *bool  `json:"is_primary"`
	TrafficLimit      int64 `json:"traffic_limit"`
	TrafficResetDay   int   `json:"traffic_reset_day"`
	TrafficUsedOffset int64 `json:"traffic_used_offset"`
}

// serverResponse is returned to the frontend (never includes auth_data).
type serverResponse struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	SSHUser     string `json:"ssh_user"`
	AuthType    string `json:"auth_type"`
	HasAuth     bool   `json:"has_auth"`
	Description string `json:"description"`
	ConfigPath  string `json:"config_path"`
	APIPort     int    `json:"api_port"`
	SingboxPort int    `json:"singbox_port"`
	SingboxUser string `json:"singbox_user"`
	Enabled     bool   `json:"enabled"`
	IsLocal     bool   `json:"is_local"`
	IsPrimary   bool   `json:"is_primary"`
	HasStatus   string `json:"has_status"`
	TrafficLimit      int64  `json:"traffic_limit"`
	TrafficResetDay   int    `json:"traffic_reset_day"`
	TrafficUsedOffset int64  `json:"traffic_used_offset"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func toServerResponse(s storage.XrayServer, hasAuth bool) serverResponse {
	return serverResponse{
		ID:          s.ID,
		Name:        s.Name,
		Host:        s.Host,
		Port:        s.Port,
		SSHUser:     s.SSHUser,
		AuthType:    s.AuthType,
		HasAuth:     hasAuth,
		Description: s.Description,
		ConfigPath:  s.ConfigPath,
		APIPort:     s.APPort,
		SingboxPort: s.SingboxPort,
		SingboxUser: s.SingboxUser,
		Enabled:     s.Enabled,
		IsLocal:     s.IsLocal,
		IsPrimary:   s.IsPrimary,
		HasStatus:   s.HasStatus,
		TrafficLimit:      s.TrafficLimit,
		TrafficResetDay:   s.TrafficResetDay,
		TrafficUsedOffset: s.TrafficUsedOffset,
		CreatedAt:   s.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   s.UpdatedAt.Format(time.RFC3339),
	}
}

func (h *serversHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.repo == nil {
		writeError(w, http.StatusInternalServerError, errors.New("repository not configured"))
		return
	}

	// Support both /api/admin/servers and /api/admin/singbox-servers prefixes
	path := r.URL.Path
	path = strings.TrimPrefix(path, "/api/admin/servers")
	path = strings.TrimPrefix(path, "/api/admin/singbox-servers")
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

func (h *serversHandler) handleList(w http.ResponseWriter, r *http.Request) {
	servers, err := h.repo.ListXrayServers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	resp := make([]serverResponse, 0, len(servers))
	for _, s := range servers {
		auth, err := h.repo.GetXrayServerAuth(r.Context(), s.ID)
		resp = append(resp, toServerResponse(s, err == nil && auth.AuthData != ""))
	}
	respondJSON(w, http.StatusOK, resp)
}

func (h *serversHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req serverRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeBadRequest(w, "无效的请求体: "+err.Error())
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	isLocal := false
	if req.IsLocal != nil {
		isLocal = *req.IsLocal
	}
	isPrimary := false
	if req.IsPrimary != nil {
		isPrimary = *req.IsPrimary
	}

		srv := storage.XrayServer{
			Name:        req.Name,
			Host:        req.Host,
			Port:        req.Port,
			SSHUser:     req.SSHUser,
			Description: req.Description,
			ConfigPath:  req.ConfigPath,
			APPort:      req.APIPort,
			SingboxPort: req.SingboxPort,
			SingboxUser: req.SingboxUser,
			Enabled:     enabled,
			IsLocal:     isLocal,
			IsPrimary:   isPrimary,
			TrafficLimit:      req.TrafficLimit,
			TrafficResetDay:   req.TrafficResetDay,
			TrafficUsedOffset: req.TrafficUsedOffset,
		}
	authData := &storage.XrayServerAuth{
		AuthType: req.AuthType,
		AuthData: req.AuthData,
	}
	created, err := h.repo.CreateXrayServer(r.Context(), &srv, authData)
	if err != nil {
		if errors.Is(err, storage.ErrXrayServerExists) {
			writeBadRequest(w, "服务器名称或主机:端口已存在")
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	respondJSON(w, http.StatusCreated, toServerResponse(created, authData.AuthData != ""))
}

func (h *serversHandler) handleItem(w http.ResponseWriter, r *http.Request, id int64, action string) {
	switch {
	case action == "" && r.Method == http.MethodGet:
		h.handleGet(w, r, id)
	case action == "" && r.Method == http.MethodPut:
		h.handleUpdate(w, r, id)
	case action == "" && r.Method == http.MethodDelete:
		h.handleDelete(w, r, id)
	case action == "test" && r.Method == http.MethodPost:
		h.handleTest(w, r, id)
	case action == "status" && r.Method == http.MethodPost:
		h.handleStatus(w, r, id)
	case action == "deploy" && r.Method == http.MethodPost:
		h.handleDeploy(w, r, id)
	case action == "restart" && r.Method == http.MethodPost:
		h.handleRestart(w, r, id)
	case action == "sync-node" && r.Method == http.MethodPost:
		h.handleSyncNode(w, r, id)
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPost)
	}
}

func (h *serversHandler) handleGet(w http.ResponseWriter, r *http.Request, id int64) {
	srv, err := h.repo.GetXrayServer(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrXrayServerNotFound) {
			http.NotFound(w, r)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	auth, err := h.repo.GetXrayServerAuth(r.Context(), id)
	respondJSON(w, http.StatusOK, toServerResponse(srv, err == nil && auth.AuthData != ""))
}

func (h *serversHandler) handleUpdate(w http.ResponseWriter, r *http.Request, id int64) {
	var req serverRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeBadRequest(w, "无效的请求体: "+err.Error())
		return
	}

	existing, err := h.repo.GetXrayServer(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrXrayServerNotFound) {
			http.NotFound(w, r)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	// Preserve fields not submitted
	if req.Name == "" {
		req.Name = existing.Name
	}
	if req.Host == "" {
		req.Host = existing.Host
	}
	if req.Port <= 0 {
		req.Port = existing.Port
	}
	if req.SSHUser == "" {
		req.SSHUser = existing.SSHUser
	}
	if req.ConfigPath == "" {
		req.ConfigPath = existing.ConfigPath
	}
	if req.APIPort <= 0 {
		req.APIPort = existing.APPort
	}
	if req.SingboxPort <= 0 {
		req.SingboxPort = existing.SingboxPort
	}
	enabled := existing.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	isLocal := existing.IsLocal
	if req.IsLocal != nil {
		isLocal = *req.IsLocal
	}
	isPrimary := existing.IsPrimary
	if req.IsPrimary != nil {
		isPrimary = *req.IsPrimary
	}

		srv := storage.XrayServer{
			ID:          id,
			Name:        req.Name,
			Host:        req.Host,
			Port:        req.Port,
			SSHUser:     req.SSHUser,
			Description: req.Description,
			ConfigPath:  req.ConfigPath,
			APPort:      req.APIPort,
			SingboxPort: req.SingboxPort,
			SingboxUser: req.SingboxUser,
			Enabled:     enabled,
			IsLocal:     isLocal,
			IsPrimary:   isPrimary,
			TrafficLimit:      req.TrafficLimit,
			TrafficResetDay:   req.TrafficResetDay,
			TrafficUsedOffset: req.TrafficUsedOffset,
		}

	var authData *storage.XrayServerAuth
	if req.AuthData != "" {
		authData = &storage.XrayServerAuth{AuthType: req.AuthType, AuthData: req.AuthData}
	}

	updated, err := h.repo.UpdateXrayServer(r.Context(), &srv, authData)
	if err != nil {
		if errors.Is(err, storage.ErrXrayServerExists) {
			writeBadRequest(w, "服务器名称或主机:端口已存在")
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	hasAuth := authData != nil
	if !hasAuth {
		// Check if existing auth is present
		a, err := h.repo.GetXrayServerAuth(r.Context(), id)
		hasAuth = err == nil && a.AuthData != ""
	}
	respondJSON(w, http.StatusOK, toServerResponse(updated, hasAuth))
}

func (h *serversHandler) handleDelete(w http.ResponseWriter, r *http.Request, id int64) {
	if err := h.repo.DeleteXrayServer(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrXrayServerNotFound) {
			http.NotFound(w, r)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

// buildOps assembles a singboxops.Client from stored server + auth data.
func (h *serversHandler) buildOps(ctx context.Context, id int64) (*singboxops.Client, *storage.XrayServer, error) {
	srv, err := h.repo.GetXrayServer(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	auth, err := h.repo.GetXrayServerAuth(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	ops, err := singboxops.New(singboxops.Remote{
		Host:        srv.Host,
		Port:        srv.Port,
		SSHUser:     srv.SSHUser,
		AuthType:    auth.AuthType,
		AuthData:    auth.AuthData,
		ConfigPath:  srv.ConfigPath,
		APIPort:     srv.APPort,
		SingboxPort: srv.SingboxPort,
		SingboxUser: srv.SingboxUser,
	})
	if err != nil {
		return nil, nil, err
	}
	return ops, &srv, nil
}

func (h *serversHandler) handleTest(w http.ResponseWriter, r *http.Request, id int64) {
	ops, _, err := h.buildOps(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := ops.TestConnection(r.Context()); err != nil {
		h.repo.SetXrayServerStatus(r.Context(), id, "连接失败: "+err.Error())
		writeError(w, http.StatusBadGateway, err)
		return
	}
	_ = h.repo.SetXrayServerStatus(r.Context(), id, "连接正常")
	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *serversHandler) handleStatus(w http.ResponseWriter, r *http.Request, id int64) {
	ops, _, err := h.buildOps(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	st, err := ops.GetStatus(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	_ = h.repo.SetXrayServerStatus(r.Context(), id, st.Message)
	respondJSON(w, http.StatusOK, st)
}

func (h *serversHandler) handleDeploy(w http.ResponseWriter, r *http.Request, id int64) {
	ops, srv, err := h.buildOps(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	content, err := ops.GenerateConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := ops.Deploy(r.Context(), content); err != nil {
		_ = h.repo.SetXrayServerStatus(r.Context(), id, "部署失败: "+err.Error())
		writeError(w, http.StatusBadGateway, err)
		return
	}
	_ = h.repo.SetXrayServerStatus(r.Context(), id, "已部署 "+time.Now().Format("15:04:05"))
	logger.Info("[Servers] 部署配置", "server_id", id, "server", srv.Name)
	respondJSON(w, http.StatusOK, map[string]bool{"deployed": true})
}

func (h *serversHandler) handleRestart(w http.ResponseWriter, r *http.Request, id int64) {
	ops, srv, err := h.buildOps(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := ops.Restart(r.Context()); err != nil {
		_ = h.repo.SetXrayServerStatus(r.Context(), id, "重启失败: "+err.Error())
		writeError(w, http.StatusBadGateway, err)
		return
	}
	_ = h.repo.SetXrayServerStatus(r.Context(), id, "已重启 "+time.Now().Format("15:04:05"))
	logger.Info("[Servers] 重启服务", "server_id", id, "server", srv.Name)
	respondJSON(w, http.StatusOK, map[string]bool{"restarted": true})
}

// handleSyncNode creates or updates a node in the nodes table for the server,
// making it available for subscription output.
func (h *serversHandler) handleSyncNode(w http.ResponseWriter, r *http.Request, id int64) {
	ops, srv, err := h.buildOps(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	clashConfig, err := ops.GenerateClashProxy()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	username := auth.UsernameFromContext(r.Context())
	if username == "" {
		writeError(w, http.StatusUnauthorized, errors.New("unauthorized"))
		return
	}

	node, err := h.repo.SyncServerNode(r.Context(), id, username, clashConfig)
	if err != nil {
		logger.Error("[Servers] 同步节点失败", "server_id", id, "error", err)
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	_ = h.repo.SetXrayServerStatus(r.Context(), id, "节点已同步")
	logger.Info("[Servers] 同步节点", "server_id", id, "server", srv.Name, "node_id", node.ID)
	respondJSON(w, http.StatusOK, map[string]any{
		"synced":    true,
		"node_id":   node.ID,
		"node_name": node.NodeName,
	})
}
