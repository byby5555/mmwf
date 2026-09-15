package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"miaomiaowux/internal/util"
)

// ChildManageHandler 处理子服务器的管理 API 请求
type ChildManageHandler struct {
	configToken string // 用于身份验证的令牌
}

// 创建一个新的子管理处理程序
func NewChildManageHandler(configToken string) *ChildManageHandler {
	return &ChildManageHandler{
		configToken: configToken,
	}
}

// 验证检查请求是否被授权
func (h *ChildManageHandler) authenticate(r *http.Request) bool {
	if h.configToken == "" {
		return true
	}

	auth := r.Header.Get("Authorization")
	if auth == "" {
		auth = r.Header.Get("MM-Remote-Token")
	}
	if auth == "" {
		return false
	}

	if strings.HasPrefix(auth, "Bearer ") {
		token := strings.TrimPrefix(auth, "Bearer ")
		return token == h.configToken
	}

	return auth == h.configToken
}

// 写入 JSON 响应
func childWriteJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// 写入错误响应
func childWriteError(w http.ResponseWriter, statusCode int, message string) {
	childWriteJSON(w, statusCode, map[string]interface{}{
		"success": false,
		"error":   message,
	})
}

// ================== 系统服务状态 ==================

// ChildServicesStatusResponse 表示服务状态的响应
type ChildServicesStatusResponse struct {
	Success bool                `json:"success"`
	Xray    *ChildServiceStatus `json:"xray,omitempty"`
	Nginx   *ChildServiceStatus `json:"nginx,omitempty"`
}

// ChildServiceStatus 代表服务状态
type ChildServiceStatus struct {
	Installed bool   `json:"installed"`
	Running   bool   `json:"running"`
	Version   string `json:"version,omitempty"`
}

func (h *ChildManageHandler) HandleServicesStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		childWriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	if !h.authenticate(r) {
		childWriteError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	response := ChildServicesStatusResponse{
		Success: true,
		Xray:    h.getSingBoxStatus(),
		Nginx:   h.getNginxStatus(),
	}

	childWriteJSON(w, http.StatusOK, response)
}

func (h *ChildManageHandler) getSingBoxStatus() *ChildServiceStatus {
	status := &ChildServiceStatus{}

	singBoxPath, err := exec.LookPath("sing-box")
	if err != nil {
		commonPaths := []string{"/usr/local/bin/sing-box", "/usr/bin/sing-box", "/opt/sing-box/sing-box"}
		for _, p := range commonPaths {
			if _, err := os.Stat(p); err == nil {
				singBoxPath = p
				break
			}
		}
	}

	if singBoxPath != "" {
		status.Installed = true
		cmd := exec.Command(singBoxPath, "version")
		output, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			if len(lines) > 0 {
				status.Version = strings.TrimSpace(lines[0])
			}
		}
	}

	cmd := exec.Command("systemctl", "is-active", "sing-box")
	output, _ := cmd.Output()
	status.Running = strings.TrimSpace(string(output)) == "active"

	return status
}

func (h *ChildManageHandler) getNginxStatus() *ChildServiceStatus {
	status := &ChildServiceStatus{}

	nginxPath, err := exec.LookPath("nginx")
	if err == nil {
		status.Installed = true
		cmd := exec.Command(nginxPath, "-v")
		output, err := cmd.CombinedOutput()
		if err == nil {
			status.Version = strings.TrimSpace(string(output))
		}
	}

	cmd := exec.Command("systemctl", "is-active", "nginx")
	output, _ := cmd.Output()
	status.Running = strings.TrimSpace(string(output)) == "active"

	return status
}

// ================== 服务控制 ==================

type ChildServiceControlRequest struct {
	Service string `json:"service"`
	Action  string `json:"action"`
}

func (h *ChildManageHandler) HandleServiceControl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		childWriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	if !h.authenticate(r) {
		childWriteError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req ChildServiceControlRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		childWriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Service != "xray" && req.Service != "nginx" {
		childWriteError(w, http.StatusBadRequest, "Invalid service. Must be 'xray' or 'nginx'")
		return
	}

	if req.Action != "start" && req.Action != "stop" && req.Action != "restart" {
		childWriteError(w, http.StatusBadRequest, "Invalid action. Must be 'start', 'stop', or 'restart'")
		return
	}

	// "xray" 在前端兼容层映射到 sing-box 服务
	svcName := req.Service
	if svcName == "xray" {
		svcName = "sing-box"
	}

	cmd := exec.Command("systemctl", req.Action, svcName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		childWriteError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to %s %s: %v - %s", req.Action, req.Service, err, string(output)))
		return
	}

	log.Printf("[Child Manage] Service %s: %s", req.Service, req.Action)

	childWriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Service %s %sed successfully", req.Service, req.Action),
	})
}

// ================== sing-box 安装 ==================

func (h *ChildManageHandler) HandleSingBoxInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		childWriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	if !h.authenticate(r) {
		childWriteError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	log.Printf("[Child Manage] Installing sing-box...")

	cmd := exec.Command("bash", "-c", `bash -c "$(curl -fsSL https://sing-box.app/deb-install.sh)"`)
	cmd.Env = os.Environ()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		log.Printf("[Child Manage] sing-box installation failed: %v, stderr: %s", err, stderr.String())
		childWriteError(w, http.StatusInternalServerError, fmt.Sprintf("Installation failed: %v", err))
		return
	}

	log.Printf("[Child Manage] sing-box installed successfully")

	childWriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "sing-box installed successfully",
		"output":  stdout.String(),
	})
}

func (h *ChildManageHandler) HandleSingBoxRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		childWriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	if !h.authenticate(r) {
		childWriteError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	log.Printf("[Child Manage] Removing sing-box...")

	// 停止并禁用服务
	exec.Command("systemctl", "stop", "sing-box").Run()
	exec.Command("systemctl", "disable", "sing-box").Run()

	var cmd *exec.Cmd
	if _, err := exec.LookPath("apt-get"); err == nil {
		cmd = exec.Command("bash", "-c", "apt-get remove -y sing-box")
	} else if _, err := exec.LookPath("yum"); err == nil {
		cmd = exec.Command("bash", "-c", "yum remove -y sing-box")
	} else if _, err := exec.LookPath("dnf"); err == nil {
		cmd = exec.Command("bash", "-c", "dnf remove -y sing-box")
	} else {
		// fallback: 手动删除二进制
		for _, p := range []string{"/usr/local/bin/sing-box", "/usr/bin/sing-box", "/opt/sing-box/sing-box"} {
			os.Remove(p)
		}
		cmd = exec.Command("rm", "-f", "/etc/systemd/system/sing-box.service")
	}

	cmd.Env = os.Environ()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		log.Printf("[Child Manage] sing-box removal failed: %v, stderr: %s", err, stderr.String())
		childWriteError(w, http.StatusInternalServerError, fmt.Sprintf("Removal failed: %v", err))
		return
	}

	exec.Command("systemctl", "daemon-reload").Run()

	log.Printf("[Child Manage] sing-box removed successfully")

	childWriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "sing-box removed successfully",
		"output":  stdout.String(),
	})
}

// ================== sing-box 配置 ==================

func (h *ChildManageHandler) HandleSingBoxConfig(w http.ResponseWriter, r *http.Request) {
	if !h.authenticate(r) {
		childWriteError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getSingBoxConfig(w, r)
	case http.MethodPost:
		h.setSingBoxConfig(w, r)
	default:
		childWriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (h *ChildManageHandler) getSingBoxConfig(w http.ResponseWriter, r *http.Request) {
	configPath := "/etc/sing-box/config.json"

	content, err := os.ReadFile(configPath)
	if err != nil {
		childWriteError(w, http.StatusNotFound, "sing-box config not found")
		return
	}

	childWriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"path":    configPath,
		"config":  string(content),
	})
}

func (h *ChildManageHandler) setSingBoxConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Config string `json:"config"`
		Path   string `json:"path,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		childWriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	var js json.RawMessage
	if err := json.Unmarshal([]byte(req.Config), &js); err != nil {
		childWriteError(w, http.StatusBadRequest, "Invalid JSON config")
		return
	}

	configPath := req.Path
	if configPath == "" {
		configPath = "/etc/sing-box/config.json"
	}

	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		childWriteError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create directory: %v", err))
		return
	}

	if err := os.WriteFile(configPath, []byte(req.Config), 0644); err != nil {
		childWriteError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to write config: %v", err))
		return
	}

	log.Printf("[Child Manage] sing-box config saved to %s", configPath)

	childWriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Config saved successfully",
		"path":    configPath,
	})
}

// ================== sing-box 系统配置 ==================

// ChildSingBoxSystemConfig 子服务器的系统配置状态
type ChildSingBoxSystemConfig struct {
	ClashAPIEnabled bool   `json:"clash_api_enabled"`
	ClashAPIListen  string `json:"clash_api_listen"` // 例如 "0.0.0.0:9090"
}

func (h *ChildManageHandler) HandleSingBoxSystemConfig(w http.ResponseWriter, r *http.Request) {
	if !h.authenticate(r) {
		childWriteError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getSingBoxSystemConfig(w, r)
	case http.MethodPost:
		h.updateSingBoxSystemConfig(w, r)
	default:
		childWriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (h *ChildManageHandler) getSingBoxSystemConfig(w http.ResponseWriter, r *http.Request) {
	configPath := h.findSingBoxConfigPath()
	if configPath == "" {
		childWriteError(w, http.StatusNotFound, "sing-box config not found")
		return
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		childWriteError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to read config: %v", err))
		return
	}

	var config map[string]interface{}
	if err := json.Unmarshal(content, &config); err != nil {
		childWriteError(w, http.StatusInternalServerError, fmt.Sprintf("Invalid JSON: %v", err))
		return
	}

	sysConfig := &ChildSingBoxSystemConfig{
		ClashAPIListen: "0.0.0.0:9090",
	}

	if experimental, ok := config["experimental"].(map[string]interface{}); ok {
		if clashAPI, ok := experimental["clash_api"].(map[string]interface{}); ok {
			sysConfig.ClashAPIEnabled = true
			if ec, ok := clashAPI["external_controller"].(string); ok {
				sysConfig.ClashAPIListen = ec
			}
		}
	}

	childWriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"config":  sysConfig,
	})
}

func (h *ChildManageHandler) updateSingBoxSystemConfig(w http.ResponseWriter, r *http.Request) {
	var req ChildSingBoxSystemConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		childWriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	configPath := h.findSingBoxConfigPath()
	if configPath == "" {
		childWriteError(w, http.StatusNotFound, "sing-box config not found")
		return
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		childWriteError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to read config: %v", err))
		return
	}

	var config map[string]interface{}
	if err := json.Unmarshal(content, &config); err != nil {
		childWriteError(w, http.StatusInternalServerError, fmt.Sprintf("Invalid JSON: %v", err))
		return
	}

	// 更新 experimental.clash_api
	if req.ClashAPIEnabled {
		experimental, ok := config["experimental"].(map[string]interface{})
		if !ok {
			experimental = map[string]interface{}{}
		}
		experimental["clash_api"] = map[string]interface{}{
			"external_controller": req.ClashAPIListen,
		}
		config["experimental"] = experimental
	} else {
		if experimental, ok := config["experimental"].(map[string]interface{}); ok {
			delete(experimental, "clash_api")
			if len(experimental) == 0 {
				delete(config, "experimental")
			} else {
				config["experimental"] = experimental
			}
		}
	}

	// 备份并保存
	backupPath := configPath + ".backup"
	if err := os.WriteFile(backupPath, content, 0644); err != nil {
		log.Printf("[Child Manage] Warning: failed to backup config: %v", err)
	}

	newContent, _ := json.MarshalIndent(config, "", "    ")
	if err := os.WriteFile(configPath, newContent, 0644); err != nil {
		childWriteError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to write config: %v", err))
		return
	}

	// 重启 sing-box
	cmd := exec.Command("systemctl", "restart", "sing-box")
	if err := cmd.Run(); err != nil {
		log.Printf("[Child Manage] Warning: failed to restart sing-box: %v", err)
	}

	log.Printf("[Child Manage] sing-box system config updated: clash_api=%v, listen=%s",
		req.ClashAPIEnabled, req.ClashAPIListen)

	childWriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "System config updated, sing-box restarted",
	})
}

// ================== Nginx 安装 ==================

func (h *ChildManageHandler) HandleNginxInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		childWriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	if !h.authenticate(r) {
		childWriteError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	log.Printf("[Child Manage] Installing Nginx...")

	var cmd *exec.Cmd
	if _, err := exec.LookPath("apt-get"); err == nil {
		cmd = exec.Command("bash", "-c", "apt-get update && apt-get install -y nginx")
	} else if _, err := exec.LookPath("yum"); err == nil {
		cmd = exec.Command("bash", "-c", "yum install -y nginx")
	} else if _, err := exec.LookPath("dnf"); err == nil {
		cmd = exec.Command("bash", "-c", "dnf install -y nginx")
	} else {
		childWriteError(w, http.StatusInternalServerError, "No supported package manager found")
		return
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		log.Printf("[Child Manage] Nginx installation failed: %v, stderr: %s", err, stderr.String())
		childWriteError(w, http.StatusInternalServerError, fmt.Sprintf("Installation failed: %v", err))
		return
	}

	log.Printf("[Child Manage] Nginx installed successfully")

	childWriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Nginx installed successfully",
		"output":  stdout.String(),
	})
}

func (h *ChildManageHandler) HandleNginxRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		childWriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	if !h.authenticate(r) {
		childWriteError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	log.Printf("[Child Manage] Removing Nginx...")

	exec.Command("systemctl", "stop", "nginx").Run()

	var cmd *exec.Cmd
	if _, err := exec.LookPath("apt-get"); err == nil {
		cmd = exec.Command("bash", "-c", "apt-get remove -y nginx nginx-common")
	} else if _, err := exec.LookPath("yum"); err == nil {
		cmd = exec.Command("bash", "-c", "yum remove -y nginx")
	} else if _, err := exec.LookPath("dnf"); err == nil {
		cmd = exec.Command("bash", "-c", "dnf remove -y nginx")
	} else {
		childWriteError(w, http.StatusInternalServerError, "No supported package manager found")
		return
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		log.Printf("[Child Manage] Nginx removal failed: %v, stderr: %s", err, stderr.String())
		childWriteError(w, http.StatusInternalServerError, fmt.Sprintf("Removal failed: %v", err))
		return
	}

	log.Printf("[Child Manage] Nginx removed successfully")

	childWriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Nginx removed successfully",
		"output":  stdout.String(),
	})
}

// ================== Nginx 配置 ==================

func (h *ChildManageHandler) HandleNginxConfig(w http.ResponseWriter, r *http.Request) {
	if !h.authenticate(r) {
		childWriteError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getNginxConfig(w, r)
	case http.MethodPost:
		h.setNginxConfig(w, r)
	default:
		childWriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (h *ChildManageHandler) getNginxConfig(w http.ResponseWriter, r *http.Request) {
	configPaths := []string{
		"/etc/nginx/nginx.conf",
		"/usr/local/nginx/nginx.conf",
	}

	var configPath string
	var content []byte
	var err error

	for _, p := range configPaths {
		content, err = os.ReadFile(p)
		if err == nil {
			configPath = p
			break
		}
	}

	if configPath == "" {
		childWriteError(w, http.StatusNotFound, "Nginx config not found")
		return
	}

	childWriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"path":    configPath,
		"config":  string(content),
	})
}

func (h *ChildManageHandler) setNginxConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Config string `json:"config"`
		Path   string `json:"path,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		childWriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	configPath := req.Path
	if configPath == "" {
		configPath = "/etc/nginx/nginx.conf"
	}

	backupPath := configPath + ".bak." + time.Now().Format("20060102150405")
	if content, err := os.ReadFile(configPath); err == nil {
		os.WriteFile(backupPath, content, 0644)
	}

	if err := os.WriteFile(configPath, []byte(req.Config), 0644); err != nil {
		childWriteError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to write config: %v", err))
		return
	}

	cmd := exec.Command("nginx", "-t")
	output, err := cmd.CombinedOutput()
	if err != nil {
		if backup, err := os.ReadFile(backupPath); err == nil {
			os.WriteFile(configPath, backup, 0644)
		}
		childWriteError(w, http.StatusBadRequest, fmt.Sprintf("Invalid nginx config: %s", string(output)))
		return
	}

	log.Printf("[Child Manage] Nginx config saved to %s", configPath)

	childWriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Config saved successfully",
		"path":    configPath,
	})
}

// ================== 系统信息 ==================

func (h *ChildManageHandler) HandleSystemInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		childWriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	if !h.authenticate(r) {
		childWriteError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	info := map[string]interface{}{
		"success": true,
	}

	if hostname, err := os.Hostname(); err == nil {
		info["hostname"] = hostname
	}

	if data, err := os.ReadFile("/proc/uptime"); err == nil {
		parts := strings.Fields(string(data))
		if len(parts) > 0 {
			info["uptime"] = parts[0]
		}
	}

	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		memInfo := make(map[string]string)
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				if key == "MemTotal" || key == "MemFree" || key == "MemAvailable" {
					memInfo[key] = value
				}
			}
		}
		info["memory"] = memInfo
	}

	if data, err := os.ReadFile("/proc/loadavg"); err == nil {
		info["loadavg"] = strings.TrimSpace(string(data))
	}

	childWriteJSON(w, http.StatusOK, info)
}

// HandleSystemNICs 列出本机可用于 sing-box 出站绑定地址的网卡地址。
func (h *ChildManageHandler) HandleSystemNICs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		childWriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	if !h.authenticate(r) {
		childWriteError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	nics, err := util.ListNICs()
	if err != nil {
		childWriteError(w, http.StatusInternalServerError, "列举网卡失败: "+err.Error())
		return
	}

	childWriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"nics":    nics,
	})
}

// ================== 配置文件管理 ==================

type ConfigFileInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time"`
}

func (h *ChildManageHandler) HandleSingBoxConfigFiles(w http.ResponseWriter, r *http.Request) {
	if !h.authenticate(r) {
		childWriteError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	switch r.Method {
	case http.MethodGet:
		file := r.URL.Query().Get("file")
		if file != "" {
			h.getSingBoxConfigFile(w, r, file)
		} else {
			h.listSingBoxConfigFiles(w, r)
		}
	case http.MethodPut, http.MethodPost:
		h.saveSingBoxConfigFile(w, r)
	default:
		childWriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (h *ChildManageHandler) listSingBoxConfigFiles(w http.ResponseWriter, r *http.Request) {
	configDirs := []string{
		"/etc/sing-box",
	}

	var files []ConfigFileInfo
	var baseDir string

	for _, dir := range configDirs {
		if _, err := os.Stat(dir); err == nil {
			baseDir = dir
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}

			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				if !strings.HasSuffix(entry.Name(), ".json") {
					continue
				}
				info, err := entry.Info()
				if err != nil {
					continue
				}
				files = append(files, ConfigFileInfo{
					Name:    entry.Name(),
					Path:    filepath.Join(dir, entry.Name()),
					Size:    info.Size(),
					ModTime: info.ModTime().Format(time.RFC3339),
				})
			}
			break
		}
	}

	childWriteJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"base_dir": baseDir,
		"files":    files,
	})
}

func (h *ChildManageHandler) getSingBoxConfigFile(w http.ResponseWriter, r *http.Request, file string) {
	file = filepath.Clean(file)

	configDirs := []string{
		"/etc/sing-box",
	}

	var filePath string
	for _, dir := range configDirs {
		candidate := filepath.Join(dir, file)
		if !strings.HasPrefix(candidate, dir) {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			filePath = candidate
			break
		}
	}

	if filePath == "" {
		childWriteError(w, http.StatusNotFound, "File not found")
		return
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		childWriteError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to read file: %v", err))
		return
	}

	childWriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"path":    filePath,
		"content": string(content),
	})
}

func (h *ChildManageHandler) saveSingBoxConfigFile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		File    string `json:"file"`
		Content string `json:"content"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		childWriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.File == "" {
		childWriteError(w, http.StatusBadRequest, "File name required")
		return
	}

	req.File = filepath.Base(req.File)
	if !strings.HasSuffix(req.File, ".json") {
		req.File += ".json"
	}

	configDirs := []string{
		"/etc/sing-box",
	}

	var configDir string
	for _, dir := range configDirs {
		if _, err := os.Stat(dir); err == nil {
			configDir = dir
			break
		}
	}

	if configDir == "" {
		configDir = "/etc/sing-box"
		if err := os.MkdirAll(configDir, 0755); err != nil {
			childWriteError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create config directory: %v", err))
			return
		}
	}

	filePath := filepath.Join(configDir, req.File)

	if strings.HasSuffix(req.File, ".json") {
		var js json.RawMessage
		if err := json.Unmarshal([]byte(req.Content), &js); err != nil {
			childWriteError(w, http.StatusBadRequest, "Invalid JSON content")
			return
		}
	}

	if err := os.WriteFile(filePath, []byte(req.Content), 0644); err != nil {
		childWriteError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to write file: %v", err))
		return
	}

	log.Printf("[Child Manage] sing-box config file saved: %s", filePath)

	childWriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "File saved successfully",
		"path":    filePath,
	})
}

// ================== Nginx 配置文件管理 ==================

func (h *ChildManageHandler) HandleNginxConfigFiles(w http.ResponseWriter, r *http.Request) {
	if !h.authenticate(r) {
		childWriteError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	switch r.Method {
	case http.MethodGet:
		file := r.URL.Query().Get("file")
		if file != "" {
			h.getNginxConfigFile(w, r, file)
		} else {
			h.listNginxConfigFiles(w, r)
		}
	case http.MethodPut, http.MethodPost:
		h.saveNginxConfigFile(w, r)
	default:
		childWriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (h *ChildManageHandler) listNginxConfigFiles(w http.ResponseWriter, r *http.Request) {
	configDirs := []struct {
		dir         string
		description string
	}{
		{"/etc/nginx", "main"},
		{"/etc/nginx/sites-available", "sites-available"},
		{"/etc/nginx/sites-enabled", "sites-enabled"},
		{"/etc/nginx/conf.d", "conf.d"},
	}

	result := make(map[string][]ConfigFileInfo)

	for _, cd := range configDirs {
		if _, err := os.Stat(cd.dir); err != nil {
			continue
		}
		entries, err := os.ReadDir(cd.dir)
		if err != nil {
			continue
		}

		var files []ConfigFileInfo
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			files = append(files, ConfigFileInfo{
				Name:    entry.Name(),
				Path:    filepath.Join(cd.dir, entry.Name()),
				Size:    info.Size(),
				ModTime: info.ModTime().Format(time.RFC3339),
			})
		}
		result[cd.description] = files
	}

	childWriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"files":   result,
	})
}

func (h *ChildManageHandler) getNginxConfigFile(w http.ResponseWriter, r *http.Request, file string) {
	file = filepath.Clean(file)

	allowedDirs := []string{
		"/etc/nginx",
		"/etc/nginx/sites-available",
		"/etc/nginx/sites-enabled",
		"/etc/nginx/conf.d",
		"/usr/local/nginx/conf",
	}

	var filePath string

	if filepath.IsAbs(file) {
		for _, dir := range allowedDirs {
			if strings.HasPrefix(file, dir) {
				if _, err := os.Stat(file); err == nil {
					filePath = file
					break
				}
			}
		}
	} else {
		for _, dir := range allowedDirs {
			candidate := filepath.Join(dir, file)
			if _, err := os.Stat(candidate); err == nil {
				filePath = candidate
				break
			}
		}
	}

	if filePath == "" {
		childWriteError(w, http.StatusNotFound, "File not found")
		return
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		childWriteError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to read file: %v", err))
		return
	}

	childWriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"path":    filePath,
		"content": string(content),
	})
}

func (h *ChildManageHandler) saveNginxConfigFile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		childWriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Path == "" {
		childWriteError(w, http.StatusBadRequest, "File path required")
		return
	}

	req.Path = filepath.Clean(req.Path)

	allowedDirs := []string{
		"/etc/nginx",
		"/usr/local/nginx/conf",
	}

	allowed := false
	for _, dir := range allowedDirs {
		if strings.HasPrefix(req.Path, dir) {
			allowed = true
			break
		}
	}

	if !allowed {
		childWriteError(w, http.StatusForbidden, "Path not allowed")
		return
	}

	if _, err := os.Stat(req.Path); err == nil {
		backupPath := req.Path + ".bak." + time.Now().Format("20060102150405")
		if content, err := os.ReadFile(req.Path); err == nil {
			os.WriteFile(backupPath, content, 0644)
		}
	}

	dir := filepath.Dir(req.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		childWriteError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create directory: %v", err))
		return
	}

	if err := os.WriteFile(req.Path, []byte(req.Content), 0644); err != nil {
		childWriteError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to write file: %v", err))
		return
	}

	cmd := exec.Command("nginx", "-t")
	output, err := cmd.CombinedOutput()
	if err != nil {
		backupPath := req.Path + ".bak." + time.Now().Format("20060102150405")[:14]
		if backup, err := os.ReadFile(backupPath); err == nil {
			os.WriteFile(req.Path, backup, 0644)
		}
		childWriteError(w, http.StatusBadRequest, fmt.Sprintf("Invalid nginx config: %s", string(output)))
		return
	}

	log.Printf("[Child Manage] Nginx config file saved: %s", req.Path)

	childWriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "File saved successfully",
		"path":    req.Path,
	})
}

// ================== sing-box 入站管理 ==================

type ChildInboundRequest struct {
	Action  string                 `json:"action"`
	Inbound map[string]interface{} `json:"inbound,omitempty"`
	Tag     string                 `json:"tag,omitempty"`
}

func (h *ChildManageHandler) HandleInbounds(w http.ResponseWriter, r *http.Request) {
	if !h.authenticate(r) {
		childWriteError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.listInbounds(w, r)
	case http.MethodPost:
		h.manageInbound(w, r)
	default:
		childWriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (h *ChildManageHandler) listInbounds(w http.ResponseWriter, r *http.Request) {
	configInbounds := h.getInboundsFromConfig()

	// 没有 gRPC 运行时,直接返回配置文件中的入站,标记为 config_only
	result := make([]map[string]interface{}, 0, len(configInbounds))
	for _, ib := range configInbounds {
		ibCopy := make(map[string]interface{})
		for k, v := range ib {
			ibCopy[k] = v
		}
		ibCopy["_runtime_status"] = "config_only"
		ibCopy["_source"] = "config"
		result = append(result, ibCopy)
	}

	childWriteJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"inbounds": result,
	})
}

func (h *ChildManageHandler) getInboundsFromConfig() []map[string]interface{} {
	configPath := h.findSingBoxConfigPath()
	if configPath == "" {
		return nil
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		log.Printf("[Child Manage] Failed to read config file: %v", err)
		return nil
	}

	var config map[string]interface{}
	if err := json.Unmarshal(content, &config); err != nil {
		log.Printf("[Child Manage] Failed to parse config: %v", err)
		return nil
	}

	rawInbounds, _ := config["inbounds"].([]interface{})
	inbounds := make([]map[string]interface{}, 0, len(rawInbounds))
	for _, ib := range rawInbounds {
		if ibMap, ok := ib.(map[string]interface{}); ok {
			inbounds = append(inbounds, ibMap)
		}
	}
	return inbounds
}

func (h *ChildManageHandler) manageInbound(w http.ResponseWriter, r *http.Request) {
	var req ChildInboundRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		childWriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action == "" {
		action = "add"
	}

	switch action {
	case "add":
		if req.Inbound == nil {
			childWriteError(w, http.StatusBadRequest, "Inbound payload is required")
			return
		}

		if err := h.persistInbound(req.Inbound); err != nil {
			childWriteError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to add inbound: %v", err))
			return
		}

		childWriteJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"message": "Inbound added successfully",
		})

	case "remove":
		if req.Tag == "" {
			childWriteError(w, http.StatusBadRequest, "Tag is required for remove action")
			return
		}

		if err := h.removeInboundFromConfig(req.Tag); err != nil {
			childWriteError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to remove inbound: %v", err))
			return
		}

		childWriteJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"message": "Inbound removed successfully",
		})

	default:
		childWriteError(w, http.StatusBadRequest, "Invalid action. Must be 'add' or 'remove'")
	}
}

// ================== sing-box 出站管理 ==================

type ChildOutboundRequest struct {
	Action   string                 `json:"action"`
	Outbound map[string]interface{} `json:"outbound,omitempty"`
	Tag      string                 `json:"tag,omitempty"`
	Tags     []string               `json:"tags,omitempty"`
}

func (h *ChildManageHandler) HandleOutbounds(w http.ResponseWriter, r *http.Request) {
	if !h.authenticate(r) {
		childWriteError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.listOutbounds(w, r)
	case http.MethodPost:
		h.manageOutbound(w, r)
	default:
		childWriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (h *ChildManageHandler) listOutbounds(w http.ResponseWriter, r *http.Request) {
	configPath := h.findSingBoxConfigPath()
	if configPath == "" {
		childWriteError(w, http.StatusNotFound, "sing-box config not found")
		return
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		childWriteError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to read config: %v", err))
		return
	}

	var config map[string]interface{}
	if err := json.Unmarshal(content, &config); err != nil {
		childWriteError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to parse config: %v", err))
		return
	}

	outbounds, _ := config["outbounds"].([]interface{})

	childWriteJSON(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"outbounds": outbounds,
	})
}

func (h *ChildManageHandler) manageOutbound(w http.ResponseWriter, r *http.Request) {
	var req ChildOutboundRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		childWriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action == "" {
		action = "add"
	}

	switch action {
	case "add":
		if req.Outbound == nil {
			childWriteError(w, http.StatusBadRequest, "Outbound payload is required")
			return
		}

		if err := h.persistOutbound(req.Outbound); err != nil {
			childWriteError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to add outbound: %v", err))
			return
		}

		childWriteJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"message": "Outbound added successfully",
		})

	case "remove":
		if req.Tag == "" {
			childWriteError(w, http.StatusBadRequest, "Tag is required for remove action")
			return
		}

		if err := h.removeOutboundFromConfig(req.Tag); err != nil {
			childWriteError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to remove outbound: %v", err))
			return
		}

		childWriteJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"message": "Outbound removed successfully",
		})

	default:
		childWriteError(w, http.StatusBadRequest, "Invalid action. Must be 'add' or 'remove'")
	}
}

// ================== sing-box 路由管理 ==================

type ChildRoutingRequest struct {
	Action  string                 `json:"action"`
	Routing map[string]interface{} `json:"routing,omitempty"`
	Rule    map[string]interface{} `json:"rule,omitempty"`
	Index   int                    `json:"index,omitempty"`
}

func (h *ChildManageHandler) HandleRouting(w http.ResponseWriter, r *http.Request) {
	if !h.authenticate(r) {
		childWriteError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getRouting(w, r)
	case http.MethodPost:
		h.manageRouting(w, r)
	default:
		childWriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (h *ChildManageHandler) getRouting(w http.ResponseWriter, r *http.Request) {
	configPath := h.findSingBoxConfigPath()
	if configPath == "" {
		childWriteError(w, http.StatusNotFound, "sing-box config not found")
		return
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		childWriteError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to read config: %v", err))
		return
	}

	var config map[string]interface{}
	if err := json.Unmarshal(content, &config); err != nil {
		childWriteError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to parse config: %v", err))
		return
	}

	routing, _ := config["route"].(map[string]interface{})
	if routing == nil {
		routing, _ = config["routing"].(map[string]interface{})
	}

	childWriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"routing": routing,
	})
}

func (h *ChildManageHandler) manageRouting(w http.ResponseWriter, r *http.Request) {
	var req ChildRoutingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		childWriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action == "" {
		action = "set"
	}

	configPath := h.findSingBoxConfigPath()
	if configPath == "" {
		childWriteError(w, http.StatusNotFound, "sing-box config not found")
		return
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		childWriteError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to read config: %v", err))
		return
	}

	var config map[string]interface{}
	if err := json.Unmarshal(content, &config); err != nil {
		childWriteError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to parse config: %v", err))
		return
	}

	// sing-box uses "route" key, but accept "routing" for backward compat
	routingKey := "route"
	if _, ok := config["routing"]; ok {
		routingKey = "routing"
	}

	switch action {
	case "set":
		if req.Routing == nil {
			childWriteError(w, http.StatusBadRequest, "Routing config is required")
			return
		}
		config[routingKey] = req.Routing

	case "add_rule":
		if req.Rule == nil {
			childWriteError(w, http.StatusBadRequest, "Rule is required")
			return
		}
		routing, _ := config[routingKey].(map[string]interface{})
		if routing == nil {
			routing = map[string]interface{}{}
		}
		rules, _ := routing["rules"].([]interface{})
		rules = append(rules, req.Rule)
		routing["rules"] = rules
		config[routingKey] = routing

	case "remove_rule":
		routing, _ := config[routingKey].(map[string]interface{})
		if routing == nil {
			childWriteError(w, http.StatusBadRequest, "No routing config found")
			return
		}
		rules, _ := routing["rules"].([]interface{})
		if req.Index < 0 || req.Index >= len(rules) {
			childWriteError(w, http.StatusBadRequest, "Invalid rule index")
			return
		}
		rules = append(rules[:req.Index], rules[req.Index+1:]...)
		routing["rules"] = rules
		config[routingKey] = routing

	default:
		childWriteError(w, http.StatusBadRequest, "Invalid action")
		return
	}

	newContent, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		childWriteError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to marshal config: %v", err))
		return
	}

	if err := os.WriteFile(configPath, newContent, 0644); err != nil {
		childWriteError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to write config: %v", err))
		return
	}

	log.Printf("[Child Manage] Routing config updated")

	childWriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Routing updated successfully. Restart sing-box to apply changes.",
	})
}

// ================== 辅助函数 ==================

func (h *ChildManageHandler) findSingBoxConfigPath() string {
	configPaths := []string{
		"/etc/sing-box/config.json",
	}

	for _, p := range configPaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func (h *ChildManageHandler) findSingBoxAPIPort() int {
	configPath := h.findSingBoxConfigPath()
	if configPath == "" {
		return 9090
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		return 9090
	}

	var config map[string]interface{}
	if err := json.Unmarshal(content, &config); err != nil {
		return 9090
	}

	// sing-box: experimental.clash_api.external_controller
	if experimental, ok := config["experimental"].(map[string]interface{}); ok {
		if clashAPI, ok := experimental["clash_api"].(map[string]interface{}); ok {
			if ec, ok := clashAPI["external_controller"].(string); ok {
				// Parse port from "0.0.0.0:9090" or ":9090" or "127.0.0.1:9090"
				if idx := strings.LastIndex(ec, ":"); idx >= 0 && idx < len(ec)-1 {
					portStr := ec[idx+1:]
					if port, err := parsePort(portStr); err == nil {
						return port
					}
				}
			}
		}
	}

	// 默认端口
	return 9090
}

func parsePort(s string) (int, error) {
	var port int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			port = port*10 + int(c-'0')
		} else {
			return 0, fmt.Errorf("invalid port character: %c", c)
		}
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("port out of range: %d", port)
	}
	return port, nil
}

func (h *ChildManageHandler) persistInbound(inbound map[string]interface{}) error {
	configPath := h.findSingBoxConfigPath()
	if configPath == "" {
		return fmt.Errorf("config file not found")
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(content, &config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	inbounds, _ := config["inbounds"].([]interface{})
	inbounds = append(inbounds, inbound)
	config["inbounds"] = inbounds

	newContent, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(configPath, newContent, 0644)
}

func (h *ChildManageHandler) removeInboundFromConfig(tag string) error {
	configPath := h.findSingBoxConfigPath()
	if configPath == "" {
		return fmt.Errorf("config file not found")
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(content, &config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	inbounds, _ := config["inbounds"].([]interface{})
	var newInbounds []interface{}
	for _, ib := range inbounds {
		inbound, ok := ib.(map[string]interface{})
		if !ok {
			newInbounds = append(newInbounds, ib)
			continue
		}
		ibTag, _ := inbound["tag"].(string)
		if ibTag != tag {
			newInbounds = append(newInbounds, ib)
		}
	}
	config["inbounds"] = newInbounds

	newContent, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(configPath, newContent, 0644)
}

func (h *ChildManageHandler) persistOutbound(outbound map[string]interface{}) error {
	configPath := h.findSingBoxConfigPath()
	if configPath == "" {
		return fmt.Errorf("config file not found")
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(content, &config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	outbounds, _ := config["outbounds"].([]interface{})
	outbounds = append(outbounds, outbound)
	config["outbounds"] = outbounds

	newContent, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(configPath, newContent, 0644)
}

func (h *ChildManageHandler) removeOutboundFromConfig(tag string) error {
	configPath := h.findSingBoxConfigPath()
	if configPath == "" {
		return fmt.Errorf("config file not found")
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(content, &config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	outbounds, _ := config["outbounds"].([]interface{})
	var newOutbounds []interface{}
	for _, ob := range outbounds {
		outbound, ok := ob.(map[string]interface{})
		if !ok {
			newOutbounds = append(newOutbounds, ob)
			continue
		}
		obTag, _ := outbound["tag"].(string)
		if obTag != tag {
			newOutbounds = append(newOutbounds, ob)
		}
	}
	config["outbounds"] = newOutbounds

	newContent, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(configPath, newContent, 0644)
}

// ================== 扫描 ==================

// ChildScanResponse 表示扫描操作的响应
type ChildScanResponse struct {
	Success             bool                     `json:"success"`
	Message             string                   `json:"message"`
	XrayRunning         bool                     `json:"xray_running"`
	XrayVersion         string                   `json:"xray_version,omitempty"`
	APIPort             int                      `json:"api_port,omitempty"`
	ConfigPath          string                   `json:"config_path,omitempty"`
	Inbounds            []map[string]interface{} `json:"inbounds,omitempty"`
	ConfigModified      bool                     `json:"config_modified,omitempty"`
	ConfigAddedSections []string                 `json:"config_added_sections,omitempty"`
}

func (h *ChildManageHandler) HandleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		childWriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	if !h.authenticate(r) {
		childWriteError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	log.Printf("[Child Manage] Scanning for sing-box process...")

	// 首先执行配置检查和补全
	configResult := h.EnsureSingBoxConfig()

	response := ChildScanResponse{
		Success: true,
		Message: "Scan completed",
	}

	if configResult.Modified {
		response.ConfigModified = true
		response.ConfigAddedSections = configResult.AddedSections
		log.Printf("[Child Manage] sing-box config auto-completed, added sections: %v", configResult.AddedSections)
		cmd := exec.Command("systemctl", "restart", "sing-box")
		if err := cmd.Run(); err != nil {
			log.Printf("[Child Manage] Failed to restart sing-box after config update: %v", err)
		} else {
			log.Printf("[Child Manage] sing-box restarted after config update")
			time.Sleep(1 * time.Second)
		}
	} else if configResult.Error != "" {
		log.Printf("[Child Manage] sing-box config check warning: %s", configResult.Error)
	}

	// 获取 sing-box 状态
	singBoxStatus := h.getSingBoxStatus()
	if singBoxStatus != nil {
		response.XrayRunning = singBoxStatus.Running
		response.XrayVersion = singBoxStatus.Version
	}

	// 查找配置和 API 端口
	configPath := h.findSingBoxConfigPath()
	if configPath != "" {
		response.ConfigPath = configPath
		response.APIPort = h.findSingBoxAPIPort()

		content, err := os.ReadFile(configPath)
		if err == nil {
			var config map[string]interface{}
			if json.Unmarshal(content, &config) == nil {
				if inbounds, ok := config["inbounds"].([]interface{}); ok {
					for _, ib := range inbounds {
						if inbound, ok := ib.(map[string]interface{}); ok {
							response.Inbounds = append(response.Inbounds, inbound)
						}
					}
				}
			}
		}
	}

	if response.XrayRunning {
		response.Message = fmt.Sprintf("sing-box is running, found %d inbound(s)", len(response.Inbounds))
		if response.ConfigModified {
			response.Message += fmt.Sprintf(", config updated: added %v", response.ConfigAddedSections)
		}
	} else if singBoxStatus != nil && singBoxStatus.Installed {
		response.Message = "sing-box is installed but not running"
	} else {
		response.Message = "sing-box is not installed"
	}

	log.Printf("[Child Manage] Scan result: %s", response.Message)

	childWriteJSON(w, http.StatusOK, response)
}

// ================== sing-box 配置自动完成 ==================

// EnsureSingBoxConfigResult 配置检查结果
type EnsureSingBoxConfigResult struct {
	ConfigPath    string   `json:"config_path"`
	Modified      bool     `json:"modified"`
	AddedSections []string `json:"added_sections,omitempty"`
	Error         string   `json:"error,omitempty"`
}

// EnsureSingBoxConfig 检查并补全 sing-box 配置
// 确保配置文件包含必要的 experimental.clash_api、log 和 direct outbound
func (h *ChildManageHandler) EnsureSingBoxConfig() *EnsureSingBoxConfigResult {
	result := &EnsureSingBoxConfigResult{}

	// 1. 查找配置文件路径
	configPath := h.findSingBoxConfigPath()
	if configPath == "" {
		result.Error = "sing-box config not found"
		return result
	}
	result.ConfigPath = configPath

	// 2. 读取现有配置
	content, err := os.ReadFile(configPath)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to read config: %v", err)
		return result
	}

	var config map[string]interface{}
	if err := json.Unmarshal(content, &config); err != nil {
		result.Error = fmt.Sprintf("Invalid JSON: %v", err)
		return result
	}

	// 3. 检查并补全各个配置项
	modified := false

	// 3.1 检查 log 配置
	if !h.hasLogSection(config) {
		h.addLogSection(config)
		result.AddedSections = append(result.AddedSections, "log")
		modified = true
	}

	// 3.2 检查 experimental.clash_api
	if !h.hasClashAPI(config) {
		h.addClashAPI(config)
		result.AddedSections = append(result.AddedSections, "clash_api")
		modified = true
	}

	// 3.3 检查 direct outbound
	if !h.hasDirectOutbound(config) {
		h.addDirectOutbound(config)
		result.AddedSections = append(result.AddedSections, "direct_outbound")
		modified = true
	}

	// 4. 如果有修改，写回配置文件
	if modified {
		backupPath := configPath + ".backup"
		if err := os.WriteFile(backupPath, content, 0644); err != nil {
			log.Printf("[Child Manage] Warning: failed to backup config: %v", err)
		}

		newContent, _ := json.MarshalIndent(config, "", "    ")
		if err := os.WriteFile(configPath, newContent, 0644); err != nil {
			result.Error = fmt.Sprintf("Failed to write config: %v", err)
			return result
		}
		result.Modified = true
		log.Printf("[Child Manage] sing-box config updated, added: %v", result.AddedSections)
	}

	return result
}

// 检查是否存在 log 配置段
func (h *ChildManageHandler) hasLogSection(config map[string]interface{}) bool {
	_, ok := config["log"].(map[string]interface{})
	return ok
}

// 添加 log 配置段
func (h *ChildManageHandler) addLogSection(config map[string]interface{}) {
	config["log"] = map[string]interface{}{
		"level": "info",
	}
}

// 检查是否存在 experimental.clash_api
func (h *ChildManageHandler) hasClashAPI(config map[string]interface{}) bool {
	experimental, ok := config["experimental"].(map[string]interface{})
	if !ok {
		return false
	}
	_, ok = experimental["clash_api"].(map[string]interface{})
	return ok
}

// 添加 experimental.clash_api
func (h *ChildManageHandler) addClashAPI(config map[string]interface{}) {
	experimental, ok := config["experimental"].(map[string]interface{})
	if !ok {
		experimental = map[string]interface{}{}
	}
	experimental["clash_api"] = map[string]interface{}{
		"external_controller": "0.0.0.0:9090",
	}
	config["experimental"] = experimental
}

// 检查是否存在 direct outbound
func (h *ChildManageHandler) hasDirectOutbound(config map[string]interface{}) bool {
	outbounds, ok := config["outbounds"].([]interface{})
	if !ok {
		return false
	}
	for _, ob := range outbounds {
		outbound, ok := ob.(map[string]interface{})
		if !ok {
			continue
		}
		if tag, _ := outbound["tag"].(string); tag == "direct" {
			return true
		}
		if typ, _ := outbound["type"].(string); typ == "direct" {
			return true
		}
	}
	return false
}

// 添加 direct outbound
func (h *ChildManageHandler) addDirectOutbound(config map[string]interface{}) {
	outbounds, ok := config["outbounds"].([]interface{})
	if !ok {
		outbounds = []interface{}{}
	}

	directOutbound := map[string]interface{}{
		"type": "direct",
		"tag":  "direct",
	}
	config["outbounds"] = append([]interface{}{directOutbound}, outbounds...)
}
