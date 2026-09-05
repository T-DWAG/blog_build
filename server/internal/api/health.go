package api

import "net/http"

// Health 是健康检查接口。不读库、不读令牌。
func Health(w http.ResponseWriter, r *http.Request) {
	WriteOK(w, map[string]bool{"ok": true})
}
