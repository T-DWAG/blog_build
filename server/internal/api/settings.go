package api

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/T-DWAG/blog_build/server/internal/store"
)

// GetSettings GET /api/admin/settings 返回四个可写 key。
func (s *Server) GetSettings(c *gin.Context) {
	settings, err := s.st.GetSettings(c.Request.Context())
	if err != nil {
		WriteErr(c.Writer, http.StatusInternalServerError, http.StatusInternalServerError, "internal error")
		return
	}
	WriteOK(c.Writer, settings)
}

// putSettingReq 是写设置的请求体。
type putSettingReq struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

// PutSettings PUT /api/admin/settings，body {key, value}。
func (s *Server) PutSettings(c *gin.Context) {
	var req putSettingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		WriteErr(c.Writer, http.StatusBadRequest, http.StatusBadRequest, "bad request")
		return
	}
	if err := s.st.PutSetting(c.Request.Context(), req.Key, req.Value); err != nil {
		writeStoreErr(c, err)
		return
	}
	WriteOK(c.Writer, map[string]bool{"ok": true})
}

// Suggestions GET /api/ai/suggestions 公开，读 KeySuggestions。
func (s *Server) Suggestions(c *gin.Context) {
	settings, err := s.st.GetSettings(c.Request.Context())
	if err != nil {
		WriteErr(c.Writer, http.StatusInternalServerError, http.StatusInternalServerError, "internal error")
		return
	}
	var suggestions []string
	if raw, ok := settings[store.KeySuggestions]; ok {
		if err := json.Unmarshal(raw, &suggestions); err != nil {
			suggestions = []string{}
		}
	}
	if suggestions == nil {
		suggestions = []string{}
	}
	WriteOK(c.Writer, suggestions)
}
