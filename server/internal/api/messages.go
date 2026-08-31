package api

import (
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/T-DWAG/blog_build/server/internal/store"
)

// clientIP 取访客 IP：X-Forwarded-For 第一段，否则 RemoteAddr。
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if first := strings.TrimSpace(strings.Split(xff, ",")[0]); first != "" {
			return first
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// submitMessageReq 是访客提交留言的请求体。IP/UA 不从这里读。
type submitMessageReq struct {
	Nickname string `json:"nickname"`
	Content  string `json:"content"`
}

// SubmitMessage POST /api/messages。先限频，false → 429。
func (s *Server) SubmitMessage(c *gin.Context) {
	ip := clientIP(c.Request)
	if !s.lim.Allow(ip) {
		WriteErr(c.Writer, http.StatusTooManyRequests, http.StatusTooManyRequests, "rate limited")
		return
	}
	var req submitMessageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		WriteErr(c.Writer, http.StatusBadRequest, http.StatusBadRequest, "bad request")
		return
	}
	m, err := s.st.InsertMessage(c.Request.Context(), req.Nickname, req.Content, ip, c.Request.UserAgent())
	if err != nil {
		writeStoreErr(c, err)
		return
	}
	WriteOK(c.Writer, m.ToPublic())
}

// ListMessages GET /api/messages 公开列表，只回已过审。
func (s *Server) ListMessages(c *gin.Context) {
	messages, err := s.st.ListApprovedMessages(c.Request.Context())
	if err != nil {
		WriteErr(c.Writer, http.StatusInternalServerError, http.StatusInternalServerError, "internal error")
		return
	}
	if messages == nil {
		messages = []store.PublicMessage{}
	}
	WriteOK(c.Writer, messages)
}

// ListMessagesAdmin GET /api/admin/messages 完整列表（含 IP/UA）。
func (s *Server) ListMessagesAdmin(c *gin.Context) {
	messages, err := s.st.ListMessagesAdmin(c.Request.Context())
	if err != nil {
		WriteErr(c.Writer, http.StatusInternalServerError, http.StatusInternalServerError, "internal error")
		return
	}
	if messages == nil {
		messages = []store.Message{}
	}
	WriteOK(c.Writer, messages)
}

// reviewMessageReq 是审核请求体。
type reviewMessageReq struct {
	Action string `json:"action"`
}

// ReviewMessage PUT /api/admin/messages/{id}，body {action: approved|rejected}。
func (s *Server) ReviewMessage(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		WriteErr(c.Writer, http.StatusBadRequest, http.StatusBadRequest, "bad request")
		return
	}
	var req reviewMessageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		WriteErr(c.Writer, http.StatusBadRequest, http.StatusBadRequest, "bad request")
		return
	}
	if err := s.st.ReviewMessage(c.Request.Context(), id, req.Action); err != nil {
		writeStoreErr(c, err)
		return
	}
	WriteOK(c.Writer, map[string]bool{"ok": true})
}

// DeleteMessage DELETE /api/admin/messages/{id}。
func (s *Server) DeleteMessage(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		WriteErr(c.Writer, http.StatusBadRequest, http.StatusBadRequest, "bad request")
		return
	}
	if err := s.st.DeleteMessage(c.Request.Context(), id); err != nil {
		writeStoreErr(c, err)
		return
	}
	WriteOK(c.Writer, map[string]bool{"ok": true})
}
