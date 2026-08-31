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

// Handler 只挂 GET /healthz，本步不挂任何业务路由。
func (s *Server) Handler() http.Handler {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.GET("/healthz", gin.WrapF(Health))
	return r
}

// Listen 启动 HTTP 监听，失败时返回 error。
func (s *Server) Listen() error {
	return http.ListenAndServe(s.cfg.Addr, s.Handler())
}
