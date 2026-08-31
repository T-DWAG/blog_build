package api

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/T-DWAG/blog_build/server/internal/adminui"
)

// adminPage 从嵌入模板渲染管理台页面。无有效 cookie → 302 /admin/login。
// name="login" 时不做校验（登录页本身对未登录用户开放）。
func (s *Server) adminPage(name string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if name != "login" {
			// 校验 cookie（复用 RequireAdmin 逻辑，但页面用 302 而非 401）
			token := ""
			if cookie, err := c.Cookie("admin_token"); err == nil {
				token = cookie
			}
			if token == "" {
				c.Redirect(http.StatusFound, "/admin/login")
				return
			}
		}
		page, err := fs.ReadFile(adminui.FS(), name+".html")
		if err != nil {
			c.String(http.StatusNotFound, "not found")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", page)
	}
}

// adminRoot /admin 重定向到文章管理页。
func (s *Server) adminRoot(c *gin.Context) {
	c.Redirect(http.StatusFound, "/admin/articles")
}

// adminLogout 清除 cookie 回登录页。
func (s *Server) adminLogout(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "admin_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	c.Redirect(http.StatusFound, "/admin/login")
}

// assetProxy 从嵌入的 adminui/static 提供字体/光标/背景图。不读磁盘、不依赖 FRONTEND_DIR。
func (s *Server) assetProxy(c *gin.Context) {
	rel := strings.TrimPrefix(c.Param("filepath"), "/")
	clean := path.Clean(rel)
	if rel == "" || clean == "." || strings.HasPrefix(clean, "..") {
		c.Status(http.StatusNotFound)
		return
	}
	if _, err := fs.Stat(adminui.Static(), clean); err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.FileFromFS(clean, http.FS(adminui.Static()))
}
