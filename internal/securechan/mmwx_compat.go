package securechan

// mmwx_clean 兼容层：main.go 使用 X25519 版安全通道 API，与 mmwf 旧版 Ed25519 API 共存。
// 旧版 API（MasterIdentity/Session/SessionCache/DeriveSession 等）仍被 handler 包中
// agent/child/ws_rpc 子系统使用，不可移除。

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// X25519 密钥对管理 (mmwx_clean 兼容)
// ---------------------------------------------------------------------------

type MasterKeyPair struct {
	PrivateKey *ecdh.PrivateKey
	PublicKey  *ecdh.PublicKey
}

// LoadOrCreateMasterKey 从文件加载或生成新的 X25519 密钥对
func LoadOrCreateMasterKey(privKeyPath string) (*MasterKeyPair, error) {
	if data, err := os.ReadFile(privKeyPath); err == nil && len(data) == 32 {
		privKey, err := ecdh.X25519().NewPrivateKey(data)
		if err != nil {
			return nil, fmt.Errorf("load master key: %w", err)
		}
		return &MasterKeyPair{
			PrivateKey: privKey,
			PublicKey:  privKey.PublicKey(),
		}, nil
	}

	privKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate master key: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(privKeyPath), 0700); err != nil {
		return nil, fmt.Errorf("create key directory: %w", err)
	}

	if err := os.WriteFile(privKeyPath, privKey.Bytes(), 0600); err != nil {
		return nil, fmt.Errorf("write master key: %w", err)
	}

	return &MasterKeyPair{
		PrivateKey: privKey,
		PublicKey:  privKey.PublicKey(),
	}, nil
}

func (k *MasterKeyPair) PublicKeyBase64B64() string {
	return base64.StdEncoding.EncodeToString(k.PublicKey.Bytes())
}

// PublicKeyBase64 兼容 mmwx_clean main.go 调用
func (k *MasterKeyPair) PublicKeyBase64() string {
	return k.PublicKeyBase64B64()
}

// ---------------------------------------------------------------------------
// 会话管理 (mmwx_clean 兼容)
// ---------------------------------------------------------------------------

type mmwxSession struct {
	sharedKey []byte
	createdAt time.Time
	expiresAt time.Time
}

type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*mmwxSession
	ttl      time.Duration
}

func NewSessionStore(ttl time.Duration) *SessionStore {
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	return &SessionStore{
		sessions: make(map[string]*mmwxSession),
		ttl:      ttl,
	}
}

func (s *SessionStore) CreateSession(sharedKey []byte) (string, error) {
	if len(sharedKey) != 32 {
		return "", fmt.Errorf("shared key must be 32 bytes, got %d", len(sharedKey))
	}

	sid, err := randomHexID(16)
	if err != nil {
		return "", err
	}

	now := time.Now()
	s.mu.Lock()
	s.sessions[sid] = &mmwxSession{
		sharedKey: sharedKey,
		createdAt: now,
		expiresAt: now.Add(s.ttl),
	}
	s.mu.Unlock()

	return sid, nil
}

func (s *SessionStore) GetSession(sid string) ([]byte, error) {
	s.mu.RLock()
	sess, ok := s.sessions[sid]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("invalid or expired secure channel session")
	}
	if time.Now().After(sess.expiresAt) {
		s.mu.Lock()
		delete(s.sessions, sid)
		s.mu.Unlock()
		return nil, fmt.Errorf("invalid or expired secure channel session")
	}
	return sess.sharedKey, nil
}

func (s *SessionStore) Cleanup() {
	s.mu.Lock()
	now := time.Now()
	for sid, sess := range s.sessions {
		if now.After(sess.expiresAt) {
			delete(s.sessions, sid)
		}
	}
	s.mu.Unlock()
}

func (s *SessionStore) StartCleanup(ctx <-chan struct{}) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx:
			return
		case <-ticker.C:
			s.Cleanup()
		}
	}
}

// ---------------------------------------------------------------------------
// ECDH 共享密钥计算 (mmwx_clean 兼容)
// ---------------------------------------------------------------------------

func DeriveSharedKey(privKey *ecdh.PrivateKey, peerPubKeyBytes []byte) ([]byte, error) {
	peerPubKey, err := ecdh.X25519().NewPublicKey(peerPubKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("parse peer public key: %w", err)
	}

	sharedSecret, err := privKey.ECDH(peerPubKey)
	if err != nil {
		return nil, fmt.Errorf("ecdh: %w", err)
	}

	h := sha256.Sum256(sharedSecret)
	return h[:32], nil
}

// ---------------------------------------------------------------------------
// AES-256-GCM 加解密 (mmwx_clean 兼容)
// ---------------------------------------------------------------------------

func EncryptB64(key, plaintext []byte) (string, error) {
	return encryptGCM(key, plaintext)
}

func DecryptB64(key []byte, encryptedB64 string) ([]byte, error) {
	return decryptGCM(key, encryptedB64)
}

func encryptGCM(key, plaintext []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	combined := append(nonce, ciphertext...)
	return base64.StdEncoding.EncodeToString(combined), nil
}

func decryptGCM(key []byte, encryptedB64 string) ([]byte, error) {
	combined, err := base64.StdEncoding.DecodeString(encryptedB64)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	ns := gcm.NonceSize()
	if len(combined) < ns+gcm.Overhead() {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce := combined[:ns]
	ciphertext := combined[ns:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// ---------------------------------------------------------------------------
// HTTP 握手 Handler (mmwx_clean 兼容)
// ---------------------------------------------------------------------------

type HandshakeRequest struct {
	ClientPublicKey string `json:"client_public_key"`
}

type HandshakeResponse struct {
	ServerPublicKey string `json:"server_public_key"`
	SessionID       string `json:"session_id"`
	AuthMode        string `json:"auth_mode"`
}

func NewHandshakeHandler(masterKey *MasterKeyPair, store *SessionStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req HandshakeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONResp(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		clientPubKeyBytes, err := base64.StdEncoding.DecodeString(req.ClientPublicKey)
		if err != nil {
			writeJSONResp(w, http.StatusBadRequest, map[string]string{"error": "invalid client public key"})
			return
		}

		sharedKey, err := DeriveSharedKey(masterKey.PrivateKey, clientPubKeyBytes)
		if err != nil {
			writeJSONResp(w, http.StatusBadRequest, map[string]string{"error": "handshake failed"})
			return
		}

		sid, err := store.CreateSession(sharedKey)
		if err != nil {
			writeJSONResp(w, http.StatusInternalServerError, map[string]string{"error": "failed to create session"})
			return
		}

		writeJSONResp(w, http.StatusOK, HandshakeResponse{
			ServerPublicKey: masterKey.PublicKeyBase64(),
			SessionID:       sid,
			AuthMode:        "X25519",
		})
	})
}

func NewServerPublicKeyHandler(masterKey *MasterKeyPair) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSONResp(w, http.StatusOK, map[string]string{
			"server_public_key": masterKey.PublicKeyBase64(),
		})
	})
}

// ---------------------------------------------------------------------------
// 中间件 (mmwx_clean 兼容)
// ---------------------------------------------------------------------------

func Middleware(store *SessionStore, skipPaths []string) func(http.Handler) http.Handler {
	skipSet := make(map[string]bool)
	for _, p := range skipPaths {
		skipSet[p] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasPrefix(r.URL.Path, "/api") {
				next.ServeHTTP(w, r)
				return
			}

			if skipSet[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			for p := range skipSet {
				if len(r.URL.Path) > len(p) && r.URL.Path[:len(p)+1] == p+"/" {
					next.ServeHTTP(w, r)
					return
				}
			}

			sid := r.Header.Get("X-Session-Id")
			if sid == "" {
				next.ServeHTTP(w, r)
				return
			}

			key, err := store.GetSession(sid)
			if err != nil {
				writeSecureChannelRequired(w)
				return
			}

			if r.Body != nil && r.ContentLength != 0 {
				plaintext, err := decryptRequestBody(key, r)
				if err != nil {
					writeSecureChannelRequired(w)
					return
				}
				r.Body = io.NopCloser(bytes.NewReader(plaintext))
				r.ContentLength = int64(len(plaintext))
			}

			ew := &encryptedResponseWriter{
				w:          w,
				key:        key,
				statusCode: http.StatusOK,
			}
			next.ServeHTTP(ew, r)
			ew.flushIfNeeded()
		})
	}
}

func writeSecureChannelRequired(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"code":  "SECURE_CHANNEL_REQUIRED",
		"error": "secure channel required",
	})
}

func decryptRequestBody(key []byte, r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	return decryptGCM(key, string(body))
}

type encryptedResponseWriter struct {
	w          http.ResponseWriter
	key        []byte
	buf        bytes.Buffer
	headerSent bool
	statusCode int
}

func (e *encryptedResponseWriter) Header() http.Header {
	return e.w.Header()
}

func (e *encryptedResponseWriter) WriteHeader(code int) {
	e.statusCode = code
}

func (e *encryptedResponseWriter) Write(data []byte) (int, error) {
	if e.w.Header().Get("X-Encrypted") == "1" {
		return e.w.Write(data)
	}
	return e.buf.Write(data)
}

func (e *encryptedResponseWriter) flushIfNeeded() {
	if e.headerSent {
		return
	}
	e.headerSent = true

	data := e.buf.Bytes()
	if len(data) == 0 {
		e.w.WriteHeader(e.statusCode)
		return
	}

	if e.w.Header().Get("X-Encrypted") == "1" {
		e.w.WriteHeader(e.statusCode)
		e.w.Write(data)
		return
	}

	encrypted, err := encryptGCM(e.key, data)
	if err != nil {
		e.w.WriteHeader(http.StatusInternalServerError)
		return
	}
	e.w.Header().Set("Content-Type", "text/plain")
	e.w.Header().Set("X-Encrypted", "1")
	e.w.WriteHeader(e.statusCode)
	e.w.Write([]byte(encrypted))
}

// ---------------------------------------------------------------------------
// 工具函数
// ---------------------------------------------------------------------------

func randomHexID(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func writeJSONResp(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
