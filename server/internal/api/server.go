package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/T-DWAG/blog_build/server/internal/config"
	"github.com/T-DWAG/blog_build/server/internal/ratelimit"
	"github.com/T-DWAG/blog_build/server/internal/store"
)

// Server 持有启动进程所需的依赖。
type Server struct {
	cfg config.Config
	st  *store.Store
	lim *ratelimit.Window
}

// New 构造一个 Server。st 不能为 nil。
func New(cfg config.Config, st *store.Store, lim *ratelimit.Window) *Server {
	if st == nil {
		panic("store is nil")
	}
	return &Server{cfg: cfg, st: st, lim: lim}
}

// allowedOrigins 是 CORS 白名单：本机联调 + 线上 Pages。
var allowedOrigins = map[string]bool{
	"http://127.0.0.1:8000":  true,
	"http://localhost:8000":  true,
	"https://T-DWAG.github.io": true,
}

// CORS 中间件：白名单内的 Origin 设 ACAO，其它不设。
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if allowedOrigins[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
			c.Header("Vary", "Origin")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// Handler 挂路由。login 不鉴权；其余 /api/admin/* 一律先过 RequireAdmin。
func (s *Server) Handler() http.Handler {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(CORS())
	r.GET("/healthz", gin.WrapF(Health))

	r.POST("/api/admin/login", s.Login)

	// 公开
	r.GET("/api/articles", s.ListArticles)
	r.GET("/api/articles/:slug", s.GetArticle)
	r.GET("/api/projects", s.ListProjects)
	r.GET("/api/messages", s.ListMessages)
	r.POST("/api/messages", s.SubmitMessage)
	r.GET("/api/search", s.Search)
	r.GET("/api/tags", s.Tags)
	r.GET("/api/ai/suggestions", s.Suggestions)

	admin := r.Group("/api/admin", s.RequireAdmin)
	admin.GET("/settings", s.GetSettings)
	admin.PUT("/settings", s.PutSettings)
	admin.POST("/preview", s.Preview)
	admin.POST("/password", s.ChangePassword)
	admin.GET("/articles", s.ListArticlesAdmin)
	admin.POST("/articles", s.CreateArticle)
	admin.PUT("/articles/:id", s.UpdateArticle)
	admin.DELETE("/articles/:id", s.DeleteArticle)
	admin.GET("/projects", s.ListProjectsAdmin)
	admin.POST("/projects", s.CreateProject)
	admin.PUT("/projects/:id", s.UpdateProject)
	admin.DELETE("/projects/:id", s.DeleteProject)
	admin.GET("/messages", s.ListMessagesAdmin)
	admin.PUT("/messages/:id", s.ReviewMessage)
	admin.DELETE("/messages/:id", s.DeleteMessage)

	// 管理台页面（cookie 校验，非 Bearer）
	r.GET("/admin", s.adminRoot)
	r.GET("/admin/login", s.adminPage("login"))
	r.GET("/admin/articles", s.adminPage("articles"))
	r.GET("/admin/articles/new", s.adminPage("article_edit"))
	r.GET("/admin/articles/:id", s.adminPage("article_edit"))
	r.GET("/admin/projects", s.adminPage("projects"))
	r.GET("/admin/messages", s.adminPage("messages"))
	r.GET("/admin/settings", s.adminPage("settings"))
	r.GET("/admin/logout", s.adminLogout)
	r.GET("/admin-assets/*filepath", s.assetProxy)
	return r
}

// Listen 启动 HTTP 监听，失败时返回 error。
func (s *Server) Listen() error {
	return http.ListenAndServe(s.cfg.Addr, s.Handler())
}
