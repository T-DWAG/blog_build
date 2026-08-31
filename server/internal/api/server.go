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

// Handler 挂路由。login 不鉴权；其余 /api/admin/* 一律先过 RequireAdmin。
func (s *Server) Handler() http.Handler {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
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
	return r
}

// Listen 启动 HTTP 监听，失败时返回 error。
func (s *Server) Listen() error {
	return http.ListenAndServe(s.cfg.Addr, s.Handler())
}
