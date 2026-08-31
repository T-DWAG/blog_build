package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/T-DWAG/blog_build/server/internal/auth"
	"github.com/T-DWAG/blog_build/server/internal/store"
)

// ctxUser 是 RequireAdmin 写入 context 的站主用户名 key。
type ctxKey string

const ctxUser ctxKey = "api.user"

// maxFailedAttempts 连错满额即锁定。
const maxFailedAttempts = 5

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Login POST /api/admin/login。判断顺序见契约 S3。
func (s *Server) Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		WriteErr(c.Writer, http.StatusBadRequest, http.StatusBadRequest, "bad request")
		return
	}

	admin, err := s.st.GetAdminByUsername(c.Request.Context(), req.Username)
	if errors.Is(err, store.ErrNotFound) {
		// 与密码错误同文案，不暴露用户是否存在
		WriteErr(c.Writer, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err != nil {
		WriteErr(c.Writer, http.StatusInternalServerError, http.StatusInternalServerError, "internal error")
		return
	}

	// 锁定期内直接拒绝，不比较密码
	if admin.LockedUntil != nil && time.Now().Before(*admin.LockedUntil) {
		WriteErr(c.Writer, http.StatusForbidden, http.StatusForbidden, "locked")
		return
	}

	if err := auth.Compare(req.Password, admin.PasswordHash); err != nil {
		_ = s.st.RecordLoginFail(c.Request.Context(), admin.ID)
		WriteErr(c.Writer, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := s.st.RecordLoginOK(c.Request.Context(), admin.ID); err != nil {
		WriteErr(c.Writer, http.StatusInternalServerError, http.StatusInternalServerError, "internal error")
		return
	}
	token, err := auth.Issue(admin.Username, s.cfg.JWTSecret)
	if err != nil {
		WriteErr(c.Writer, http.StatusInternalServerError, http.StatusInternalServerError, "internal error")
		return
	}
	WriteOK(c.Writer, map[string]string{"token": token})
}

// RequireAdmin 校验 Authorization: Bearer <token>，失败 401。
// 成功后用户名写入 context，后续 handler 用 usernameFrom(c) 读取。
func (s *Server) RequireAdmin(c *gin.Context) {
	header := c.GetHeader("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		WriteErr(c.Writer, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized")
		c.Abort()
		return
	}
	token := strings.TrimPrefix(header, "Bearer ")
	username, err := auth.Parse(token, s.cfg.JWTSecret)
	if err != nil {
		WriteErr(c.Writer, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized")
		c.Abort()
		return
	}
	// 用户名写入 request context（契约：ctx key api.ctxUser）
	ctx := context.WithValue(c.Request.Context(), ctxUser, username)
	c.Request = c.Request.WithContext(ctx)
	c.Next()
}

// usernameFrom 取 RequireAdmin 写入的用户名。
func usernameFrom(ctx context.Context) string {
	u, _ := ctx.Value(ctxUser).(string)
	return u
}

// AdminPing GET /api/admin/ping。S4 可删。
func (s *Server) AdminPing(c *gin.Context) {
	WriteOK(c.Writer, map[string]string{"user": usernameFrom(c.Request.Context())})
}
