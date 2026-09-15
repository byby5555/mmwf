package handler

import (
	"context"
	"encoding/json"
	"fmt"
	stdhttp "net/http"
	"time"

	"miaomiaowux/internal/auth"
	"miaomiaowux/internal/storage"
)

type XrayHandler struct {
	repo       *storage.TrafficRepository
	httpClient *stdhttp.Client
}

func NewXrayHandler(repo *storage.TrafficRepository) *XrayHandler {
	return &XrayHandler{
		repo: repo,
		httpClient: &stdhttp.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// XrayClient 表示与 sing-box Clash API 的连接
type XrayClient struct {
	apiURL string
	client *stdhttp.Client
}

// 建立与 sing-box Clash API 的连接
func (h *XrayHandler) ConnectToXray(ctx context.Context, host string, port int) (*XrayClient, error) {
	apiURL := fmt.Sprintf("http://%s:%d", host, port)
	return &XrayClient{
		apiURL: apiURL,
		client: &stdhttp.Client{Timeout: 10 * time.Second},
	}, nil
}

// 请求/响应结构
type AddOutboundRequest struct {
	Tag     string                 `json:"tag"`
	Type    string                 `json:"type"`
	Options map[string]interface{} `json:"options"`
}

type AddOutboundResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type RemoveOutboundRequest struct {
	Tag string `json:"tag"`
}

type ListOutboundsRequest struct{}

type OutboundInfo struct {
	Tag string `json:"tag"`
}

type ListOutboundsResponse struct {
	Success   bool           `json:"success"`
	Message   string         `json:"message"`
	Outbounds []OutboundInfo `json:"outbounds"`
}

type StatsRequest struct {
	Name  string `json:"name"`
	Reset bool   `json:"reset"`
}

type StatsResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Value   int64  `json:"value,omitempty"`
}

type SystemStatsResponse struct {
	Success bool                   `json:"success"`
	Message string                 `json:"message"`
	Stats   map[string]interface{} `json:"stats,omitempty"`
}

// HTTP 处理程序

func (h *XrayHandler) AddOutbound(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if r.Method != stdhttp.MethodPost {
		stdhttp.Error(w, "Method not allowed", stdhttp.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	username := auth.UsernameFromContext(ctx)
	if username == "" {
		stdhttp.Error(w, "Unauthorized", stdhttp.StatusUnauthorized)
		return
	}

	var req AddOutboundRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		stdhttp.Error(w, "Invalid request body", stdhttp.StatusBadRequest)
		return
	}

	// 从用户设置获取连接设置或使用默认值
	xrayHost, xrayPort, err := h.getXraySettings(ctx, username)
	if err != nil {
		stdhttp.Error(w, err.Error(), stdhttp.StatusInternalServerError)
		return
	}

	// sing-box 使用配置文件管理出站，Clash API 不直接支持添加出站
	_ = xrayHost
	_ = xrayPort

	// 这里返回提示信息，实际出站管理通过配置文件操作
	response := AddOutboundResponse{
		Success: false,
		Message: "Outbound management via Clash API is not supported in sing-box mode. Use config file operations instead.",
	}

	h.writeJSONResponse(w, response)
}

func (h *XrayHandler) RemoveOutbound(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if r.Method != stdhttp.MethodDelete {
		stdhttp.Error(w, "Method not allowed", stdhttp.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	username := auth.UsernameFromContext(ctx)
	if username == "" {
		stdhttp.Error(w, "Unauthorized", stdhttp.StatusUnauthorized)
		return
	}

	var req RemoveOutboundRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		stdhttp.Error(w, "Invalid request body", stdhttp.StatusBadRequest)
		return
	}

	if req.Tag == "" {
		stdhttp.Error(w, "Tag is required", stdhttp.StatusBadRequest)
		return
	}

	response := AddOutboundResponse{
		Success: false,
		Message: "Outbound management via Clash API is not supported in sing-box mode. Use config file operations instead.",
	}

	h.writeJSONResponse(w, response)
}

func (h *XrayHandler) ListOutbounds(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if r.Method != stdhttp.MethodGet {
		stdhttp.Error(w, "Method not allowed", stdhttp.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	username := auth.UsernameFromContext(ctx)
	if username == "" {
		stdhttp.Error(w, "Unauthorized", stdhttp.StatusUnauthorized)
		return
	}

	// 获取连接设置
	xrayHost, xrayPort, err := h.getXraySettings(ctx, username)
	if err != nil {
		stdhttp.Error(w, err.Error(), stdhttp.StatusInternalServerError)
		return
	}

	xrayClient, err := h.ConnectToXray(ctx, xrayHost, xrayPort)
	if err != nil {
		h.writeErrorResponse(w, "Failed to connect to Clash API", err)
		return
	}

	// 通过 Clash API 获取出站列表
	url := fmt.Sprintf("%s/proxies", xrayClient.apiURL)
	httpReq, _ := stdhttp.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := xrayClient.client.Do(httpReq)
	if err != nil {
		h.writeErrorResponse(w, "Failed to query Clash API", err)
		return
	}
	defer resp.Body.Close()

	var apiResp struct {
		Proxies map[string]interface{} `json:"proxies"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		h.writeErrorResponse(w, "Failed to decode Clash API response", err)
		return
	}

	outbounds := make([]OutboundInfo, 0, len(apiResp.Proxies))
	for tag := range apiResp.Proxies {
		outbounds = append(outbounds, OutboundInfo{Tag: tag})
	}

	response := ListOutboundsResponse{
		Success:   true,
		Message:   "Success",
		Outbounds: outbounds,
	}

	h.writeJSONResponse(w, response)
}

func (h *XrayHandler) GetStats(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if r.Method != stdhttp.MethodPost {
		stdhttp.Error(w, "Method not allowed", stdhttp.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	username := auth.UsernameFromContext(ctx)
	if username == "" {
		stdhttp.Error(w, "Unauthorized", stdhttp.StatusUnauthorized)
		return
	}

	var req StatsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		stdhttp.Error(w, "Invalid request body", stdhttp.StatusBadRequest)
		return
	}

	if req.Name == "" {
		stdhttp.Error(w, "Stats name is required", stdhttp.StatusBadRequest)
		return
	}

	// 获取连接设置
	xrayHost, xrayPort, err := h.getXraySettings(ctx, username)
	if err != nil {
		stdhttp.Error(w, err.Error(), stdhttp.StatusInternalServerError)
		return
	}

	xrayClient, err := h.ConnectToXray(ctx, xrayHost, xrayPort)
	if err != nil {
		h.writeErrorResponse(w, "Failed to connect to Clash API", err)
		return
	}

	// 通过 Clash API 获取连接统计
	url := fmt.Sprintf("%s/connections", xrayClient.apiURL)
	httpReq, _ := stdhttp.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := xrayClient.client.Do(httpReq)
	if err != nil {
		h.writeErrorResponse(w, "Failed to query Clash API", err)
		return
	}
	defer resp.Body.Close()

	var apiResp struct {
		Upload   int64 `json:"uploadTotal"`
		Download int64 `json:"downloadTotal"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		h.writeErrorResponse(w, "Failed to decode Clash API response", err)
		return
	}

	var value int64
	if req.Name == "upload" || req.Name == "up" {
		value = apiResp.Upload
	} else if req.Name == "download" || req.Name == "down" {
		value = apiResp.Download
	} else {
		value = apiResp.Upload + apiResp.Download
	}

	response := StatsResponse{
		Success: true,
		Message: "Success",
		Value:   value,
	}

	h.writeJSONResponse(w, response)
}

func (h *XrayHandler) GetSystemStats(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if r.Method != stdhttp.MethodGet {
		stdhttp.Error(w, "Method not allowed", stdhttp.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	username := auth.UsernameFromContext(ctx)
	if username == "" {
		stdhttp.Error(w, "Unauthorized", stdhttp.StatusUnauthorized)
		return
	}

	// 获取连接设置
	xrayHost, xrayPort, err := h.getXraySettings(ctx, username)
	if err != nil {
		stdhttp.Error(w, err.Error(), stdhttp.StatusInternalServerError)
		return
	}

	xrayClient, err := h.ConnectToXray(ctx, xrayHost, xrayPort)
	if err != nil {
		h.writeErrorResponse(w, "Failed to connect to Clash API", err)
		return
	}

	// 通过 Clash API 获取系统统计
	url := fmt.Sprintf("%s/connections", xrayClient.apiURL)
	httpReq, _ := stdhttp.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := xrayClient.client.Do(httpReq)
	if err != nil {
		h.writeErrorResponse(w, "Failed to query Clash API", err)
		return
	}
	defer resp.Body.Close()

	var stats map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		h.writeErrorResponse(w, "Failed to decode Clash API response", err)
		return
	}

	response := SystemStatsResponse{
		Success: true,
		Message: "Success",
		Stats:   stats,
	}

	h.writeJSONResponse(w, response)
}

// 辅助函数

func (h *XrayHandler) getXraySettings(ctx context.Context, username string) (string, int, error) {
	// 默认设置 — sing-box Clash API 默认端口 9090
	defaultHost := "127.0.0.1"
	defaultPort := 9090

	if h.repo == nil {
		return defaultHost, defaultPort, nil
	}

	host := defaultHost
	port := defaultPort

	return host, port, nil
}

func (h *XrayHandler) writeErrorResponse(w stdhttp.ResponseWriter, message string, err error) {
	response := AddOutboundResponse{
		Success: false,
		Message: fmt.Sprintf("%s: %v", message, err),
	}
	h.writeJSONResponse(w, response)
}

func (h *XrayHandler) writeJSONResponse(w stdhttp.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(stdhttp.StatusOK)
	json.NewEncoder(w).Encode(data)
}
