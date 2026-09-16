// securechan_user.go — 新前端 (mmwx-pro) 的 v2 加密通道 + v3u/v3 统一分发端点。
//
// 协议 (与前端 assets/index-*.js 中混淆逻辑对齐, 已在 162 参照机实测验证):
//   1. 握手: POST /api/securechan/handshake?t=<ts>
//      请求: { "client_pub_b64": "<base64 32B X25519>", "origin": "<window.location.origin>", "proto": "v2" }
//      响应: { "proto": "v2", "runtime_proof": null, "server_pub_b64": "...", "session_id": "...", "backend_proof": null }
//   2. 密钥派生: X25519 ECDH → HKDF-SHA256(info="securechan-v2\n"+sessionID, salt=nil) → 双向 AES-256-GCM
//      前 32B: server→client key; 次 32B: client→server key; 再各 12B nonce
//      (前端 isMaster=false 对应服务端 isMaster=true)
//   3. 业务请求: POST /api/v3u (user) 或 /api/v3 (admin)
//      headers: x-secure-channel: v1, x-session-id: <sid>, content-type: text/plain; charset=utf-8
//      body = base64(envelope), envelope = [0x01][seq:8BE][AES-GCM ct+tag]
//      解密明文 = { "op": "<16位hex>", "payload": {...}, "p": [...], "q": "..." }
//      op 路由映射表见 v3_routing.go (从官方 bundle 逆向生成)
//   4. 响应: 同样加密, header X-Secure-Channel: v1
//
// Session TTL 30 分钟。runtime_proof / backend_proof 返回 null (前端 mm 函数已 patch 为 no-op,
// backend_proof 为 null 时跳过 fm 检查)。

package handler

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"miaomiaowux/internal/securechan"
)

// 协议常量(必须与前端混淆 JS 保持一致)
const (
	secureChannelVersion       = "v1"
	headerSecureChannel        = "X-Secure-Channel"
	headerSessionID            = "X-Session-Id"
	headerSecureChannelExpired = "X-Secure-Channel-Expired"
	userSessionTTL             = 30 * time.Minute
	maxEncryptedBodyBytes      = 8 << 20 // 8 MiB, 防 DoS
	sessionIDBytes             = 16      // 128-bit session id
	handshakeProtoV2           = "v2"
)

// UserSecureChannelHandler 管理前端会话池 + 提供握手/分发 endpoint。
type UserSecureChannelHandler struct {
	sessions sync.Map // sessionID(hex string) -> *userSession
}

type userSession struct {
	sess      *securechan.Session
	createdAt time.Time
	lastUsed  time.Time
	mu        sync.Mutex // 串行化 Encrypt/Decrypt
}

// NewUserSecureChannelHandler 创建会话池并启动后台清理。
func NewUserSecureChannelHandler() *UserSecureChannelHandler {
	h := &UserSecureChannelHandler{}
	go h.cleanupLoop()
	return h
}

// Handshake POST /api/securechan/handshake
// v2 请求: { "client_pub_b64": "<base64 32B X25519>", "origin": "...", "proto": "v2" }
// v2 响应: { "proto":"v2", "runtime_proof":null, "server_pub_b64":"...", "session_id":"...", "backend_proof":null }
func (h *UserSecureChannelHandler) Handshake(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ClientPubB64 string `json:"client_pub_b64"`
		Origin       string `json:"origin"`
		Proto        string `json:"proto"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	clientPub, err := base64.StdEncoding.DecodeString(req.ClientPubB64)
	if err != nil || len(clientPub) != 32 {
		http.Error(w, "invalid client_pub_b64", http.StatusBadRequest)
		return
	}

	// 只支持 v2 (新前端协议)。v1 已被官方前端废弃 (proto 非 v2 时前端直接 throw)。
	if req.Proto != handshakeProtoV2 {
		log.Printf("[securechan_user] handshake rejected: unsupported proto %q", req.Proto)
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "unsupported protocol version",
			"proto": req.Proto,
		})
		return
	}

	serverPriv, serverPub, err := securechan.GenerateEphemeral()
	if err != nil {
		http.Error(w, "keygen failed", http.StatusInternalServerError)
		return
	}
	shared, err := securechan.ComputeSharedSecret(serverPriv, clientPub)
	if err != nil {
		http.Error(w, "ECDH failed", http.StatusInternalServerError)
		return
	}

	// 先生成 session_id, 再用它参与 HKDF info — 与前端派生顺序一致。
	sid := newSessionID()

	sess, err := securechan.DeriveSessionV2(shared, sid, true)
	if err != nil {
		http.Error(w, "session derive failed", http.StatusInternalServerError)
		return
	}

	now := time.Now()
	h.sessions.Store(sid, &userSession{sess: sess, createdAt: now, lastUsed: now})

	log.Printf("[securechan_user] v2 handshake ok: sid=%s origin=%s", sid, req.Origin)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"proto":          "v2",
		"runtime_proof":  nil,
		"server_pub_b64": base64.StdEncoding.EncodeToString(serverPub),
		"session_id":     sid,
		"backend_proof":  nil,
	})
}

// loadSession 取会话并刷新 lastUsed。找不到或已过期时返回 nil。
func (h *UserSecureChannelHandler) loadSession(sid string) *userSession {
	entryAny, ok := h.sessions.Load(sid)
	if !ok {
		return nil
	}
	entry := entryAny.(*userSession)
	if time.Since(entry.lastUsed) > userSessionTTL {
		h.sessions.Delete(sid)
		return nil
	}
	return entry
}

// decryptEnvelopeBody 读取 base64 envelope body 并解密。
func (h *UserSecureChannelHandler) decryptEnvelopeBody(entry *userSession, r *http.Request) ([]byte, error) {
	bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, maxEncryptedBodyBytes))
	r.Body.Close()
	if err != nil {
		return nil, err
	}
	envelope, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(bodyBytes)))
	if err != nil {
		log.Printf("[securechan_user] base64 decode failed for sid=%s len=%d err=%v",
			safePrefix(r.Header.Get(headerSessionID), 8), len(bodyBytes), err)
		return nil, err
	}

	entry.mu.Lock()
	plaintext, err := entry.sess.Decrypt(envelope)
	entry.mu.Unlock()
	if err != nil {
		log.Printf("[securechan_user] decrypt failed for sid=%s path=%s method=%s envelope_len=%d err=%v",
			safePrefix(r.Header.Get(headerSessionID), 8), r.URL.Path, r.Method, len(envelope), err)
		return nil, err
	}
	return plaintext, nil
}

// encryptEnvelopeBody 加密明文并编码为 base64。
func (h *UserSecureChannelHandler) encryptEnvelopeBody(entry *userSession, plaintext []byte) (string, error) {
	entry.mu.Lock()
	envelope, err := entry.sess.Encrypt(plaintext)
	entry.mu.Unlock()
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(envelope), nil
}

// sendSessionExpired 会话失效 — 前端看到 412 + X-Secure-Channel-Expired 会自动重新握手。
func (h *UserSecureChannelHandler) sendSessionExpired(w http.ResponseWriter) {
	w.Header().Set(headerSecureChannelExpired, "1")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusPreconditionFailed)
	_, _ = w.Write([]byte(`{"error":"secure channel session expired","code":"session_expired"}`))
}

// sendEncryptedJSON 用会话加密一个 JSON 响应并写回 (X-Secure-Channel: v1)。
func (h *UserSecureChannelHandler) sendEncryptedJSON(entry *userSession, w http.ResponseWriter, status int, v any) {
	plaintext, err := json.Marshal(v)
	if err != nil {
		http.Error(w, "marshal failed", http.StatusInternalServerError)
		return
	}
	cipherB64, err := h.encryptEnvelopeBody(entry, plaintext)
	if err != nil {
		log.Printf("[securechan_user] encrypt response failed: %v", err)
		http.Error(w, "encrypt response failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set(headerSecureChannel, secureChannelVersion)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(cipherB64))
}

// cleanupLoop 每 5 分钟扫一次过期 session
func (h *UserSecureChannelHandler) cleanupLoop() {
	t := time.NewTicker(5 * time.Minute)
	defer t.Stop()
	for range t.C {
		now := time.Now()
		h.sessions.Range(func(k, v any) bool {
			e := v.(*userSession)
			if now.Sub(e.lastUsed) > userSessionTTL {
				h.sessions.Delete(k)
			}
			return true
		})
	}
}

func newSessionID() string {
	b := make([]byte, sessionIDBytes)
	if _, err := rand.Read(b); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format("20060102150405000000000")))
	}
	return hex.EncodeToString(b)
}

// V3Request is the decrypted plaintext body of a /api/v3u or /api/v3 request.
type V3Request struct {
	Op      string          `json:"op"`
	Payload json.RawMessage `json:"payload"`
	P       []string        `json:"p"` // path params
	Q       string          `json:"q"`  // query string (raw)
}

// HandleV3 是 /api/v3u (isAdmin=false) 与 /api/v3 (isAdmin=true) 的入口。
func (h *UserSecureChannelHandler) HandleV3(w http.ResponseWriter, r *http.Request, isAdmin bool) {
	h.handleV3Request(w, r, isAdmin)
}

// handleV3Request 处理 v3u/v3 通用流程: 验 session → 解密 body → 解析 op → 分发。
func (h *UserSecureChannelHandler) handleV3Request(w http.ResponseWriter, r *http.Request, isAdmin bool) {
	sid := r.Header.Get(headerSessionID)
	entry := h.loadSession(sid)
	if entry == nil {
		h.sendSessionExpired(w)
		return
	}

	plaintext, err := h.decryptEnvelopeBody(entry, r)
	if err != nil {
		http.Error(w, "decrypt failed", http.StatusBadRequest)
		return
	}

	entry.lastUsed = time.Now()

	var v3req V3Request
	if err := json.Unmarshal(plaintext, &v3req); err != nil {
		log.Printf("[v3] bad plaintext JSON for sid=%s: %v (len=%d)", safePrefix(sid, 8), err, len(plaintext))
		h.sendEncryptedJSON(entry, w, http.StatusBadRequest, map[string]any{
			"success": false,
			"error":   "invalid v3 request body",
		})
		return
	}

	target, ok := ResolveV3Op(v3req.Op, isAdmin)
	if !ok {
		log.Printf("[v3] unknown op: %q isAdmin=%v", v3req.Op, isAdmin)
		h.sendEncryptedJSON(entry, w, http.StatusNotFound, map[string]any{
			"success": false,
			"error":   "unknown op",
		})
		return
	}

	// 构造内部请求: 方法 + 路径 + query + body
	inbound := buildInboundRequest(r, target, v3req)

	// 分发到内部 mux (复用现有全部 handler)
	recorder := &v3ResponseRecorder{
		header: http.Header{},
		body:   &bytes.Buffer{},
		status: http.StatusOK,
	}
	V3Dispatch(recorder, inbound)

	// 加密响应写回
	respPayload, err := buildV3ResponsePayload(recorder)
	if err != nil {
		log.Printf("[v3] build response payload failed: %v", err)
		h.sendEncryptedJSON(entry, w, http.StatusInternalServerError, map[string]any{
			"success": false,
			"error":   "internal error",
		})
		return
	}
	h.sendEncryptedJSON(entry, w, recorder.status, respPayload)
}

// buildInboundRequest 把 v3 请求转成标准 http.Request 喂给内部路由。
// token 从原请求的 Authorization header / cookie 透传 (登录后前端带上)。
func buildInboundRequest(orig *http.Request, target V3Target, v3req V3Request) *http.Request {
	url := target.Path
	if v3req.Q != "" {
		url += "?" + v3req.Q
	}

	inbound := (&http.Request{
		Method: target.Method,
		URL:    parseURLForV3(url),
		Header: orig.Header.Clone(),
		Body:   http.NoBody,
		Host:   orig.Host,
		RemoteAddr: orig.RemoteAddr,
	}).WithContext(orig.Context())

	if len(v3req.Payload) > 0 && string(v3req.Payload) != "null" {
		inbound.Body = io.NopCloser(bytes.NewReader(v3req.Payload))
		inbound.ContentLength = int64(len(v3req.Payload))
		inbound.Header.Set("Content-Type", "application/json")
	}

	return inbound
}

// v3ResponseRecorder 捕获内部 handler 响应。
type v3ResponseRecorder struct {
	header http.Header
	body   *bytes.Buffer
	status int
}

func (r *v3ResponseRecorder) Header() http.Header       { return r.header }
func (r *v3ResponseRecorder) WriteHeader(code int)      { r.status = code }
func (r *v3ResponseRecorder) Write(b []byte) (int, error) { return r.body.Write(b) }

// buildV3ResponsePayload 把内部 handler 的响应包装成前端期望的结构。
// 前端 fetch wrapper 会按 content-type 解析: JSON 直接解析, 文本原样返回。
func buildV3ResponsePayload(rec *v3ResponseRecorder) (any, error) {
	contentType := rec.header.Get("Content-Type")
	body := rec.body.Bytes()

	if strings.HasPrefix(contentType, "application/json") || json.Valid(body) {
		var parsed any
		if err := json.Unmarshal(body, &parsed); err == nil {
			return parsed, nil
		}
	}
	return map[string]any{
		"success": false,
		"error":   "non-JSON response",
		"data":    string(body),
	}, nil
}
