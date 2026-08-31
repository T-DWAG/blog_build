package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/T-DWAG/blog_build/server/internal/config"
)

// Server 持有启动进程所需的依赖。S01 只持有配置。
type Server struct {
	cfg config.Config
}

// New 构造一个 Server。
func New(cfg config.Config) *Server {
	return &Server{cfg: cfg}
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
