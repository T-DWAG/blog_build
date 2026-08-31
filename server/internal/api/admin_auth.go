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
	// 管理台页面走 cookie（HttpOnly）。Path=/ 覆盖 /api/admin/* 与 /admin/* 两类路径。
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "admin_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	WriteOK(c.Writer, map[string]string{"token": token})
}

// RequireAdmin 校验令牌。优先级：Authorization: Bearer > cookie admin_token。
// 失败 401；成功后用户名写入 request context。
func (s *Server) RequireAdmin(c *gin.Context) {
	token := ""
	if header := c.GetHeader("Authorization"); strings.HasPrefix(header, "Bearer ") {
		token = strings.TrimPrefix(header, "Bearer ")
	} else if cookie, err := c.Cookie("admin_token"); err == nil {
		token = cookie
	}
	if token == "" {
		WriteErr(c.Writer, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized")
		c.Abort()
		return
	}
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

// changePasswordReq 是改密码请求体。
type changePasswordReq struct {
	Old string `json:"old"`
	New string `json:"new"`
}

// ChangePassword POST /api/admin/password。旧密码错误 → 401。
func (s *Server) ChangePassword(c *gin.Context) {
	username := usernameFrom(c.Request.Context())
	if username == "" {
		WriteErr(c.Writer, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req changePasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		WriteErr(c.Writer, http.StatusBadRequest, http.StatusBadRequest, "bad request")
		return
	}
	admin, err := s.st.GetAdminByUsername(c.Request.Context(), username)
	if err != nil {
		WriteErr(c.Writer, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err := auth.Compare(req.Old, admin.PasswordHash); err != nil {
		WriteErr(c.Writer, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized")
		return
	}
	hash, err := auth.Hash(req.New)
	if err != nil {
		WriteErr(c.Writer, http.StatusInternalServerError, http.StatusInternalServerError, "internal error")
		return
	}
	if err := s.st.UpdateAdminPassword(c.Request.Context(), admin.ID, hash); err != nil {
		WriteErr(c.Writer, http.StatusInternalServerError, http.StatusInternalServerError, "internal error")
		return
	}
	WriteOK(c.Writer, map[string]bool{"ok": true})
}
