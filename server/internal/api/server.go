package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/T-DWAG/blog_build/server/internal/config"
	"github.com/T-DWAG/blog_build/server/internal/store"
)

// Server 持有启动进程所需的依赖。
type Server struct {
	cfg config.Config
	st  *store.Store
}

// New 构造一个 Server。st 不能为 nil。
func New(cfg config.Config, st *store.Store) *Server {
	if st == nil {
		panic("store is nil")
	}
	return &Server{cfg: cfg, st: st}
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

	admin := r.Group("/api/admin", s.RequireAdmin)
	admin.GET("/articles", s.ListArticlesAdmin)
	admin.POST("/articles", s.CreateArticle)
	admin.PUT("/articles/:id", s.UpdateArticle)
	admin.DELETE("/articles/:id", s.DeleteArticle)
	return r
}

// Listen 启动 HTTP 监听，失败时返回 error。
func (s *Server) Listen() error {
	return http.ListenAndServe(s.cfg.Addr, s.Handler())
}
