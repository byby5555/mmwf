package handler

import (
	"encoding/json"
	"net/http"
)

// writeJSON 写一份成功响应(任意结构)。
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
