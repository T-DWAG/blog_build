package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/T-DWAG/blog_build/server/internal/store"
)

// Search GET /api/search?q= 公开搜索，空 q 也 200（返回空两组）。
func (s *Server) Search(c *gin.Context) {
	res, err := s.st.Search(c.Request.Context(), c.Query("q"))
	if err != nil {
		WriteErr(c.Writer, http.StatusInternalServerError, http.StatusInternalServerError, "internal error")
		return
	}
	WriteOK(c.Writer, res.Normalized())
}

// Tags GET /api/tags 公开标签聚合。
func (s *Server) Tags(c *gin.Context) {
	tags, err := s.st.ListTagCounts(c.Request.Context())
	if err != nil {
		WriteErr(c.Writer, http.StatusInternalServerError, http.StatusInternalServerError, "internal error")
		return
	}
	if tags == nil {
		tags = []store.TagCount{}
	}
	WriteOK(c.Writer, tags)
}
