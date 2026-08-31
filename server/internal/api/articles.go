package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/T-DWAG/blog_build/server/internal/store"
)

// articleReq 是管理台写文章的请求体。
type articleReq struct {
	Slug      string   `json:"slug"`
	Title     string   `json:"title"`
	Summary   string   `json:"summary"`
	ContentMD string   `json:"content_md"`
	Status    string   `json:"status"`
	IsPinned  bool     `json:"is_pinned"`
	Tags      []string `json:"tags"`
	CoverURL  string   `json:"cover_url"`
}

// toArticle 把请求体转成 store.Article（ID 由调用方补）。
func (r articleReq) toArticle() *store.Article {
	return &store.Article{
		Slug:      r.Slug,
		Title:     r.Title,
		Summary:   r.Summary,
		ContentMD: r.ContentMD,
		Status:    r.Status,
		IsPinned:  r.IsPinned,
		Tags:      r.Tags,
		CoverURL:  r.CoverURL,
	}
}

// ListArticles GET /api/articles 公开列表，?tag=&page=。
func (s *Server) ListArticles(c *gin.Context) {
	tag := c.Query("tag")
	page, err := strconv.Atoi(c.Query("page"))
	if err != nil {
		page = 1
	}
	articles, err := s.st.ListPublishedArticles(c.Request.Context(), tag, page)
	if err != nil {
		WriteErr(c.Writer, http.StatusInternalServerError, http.StatusInternalServerError, "internal error")
		return
	}
	if articles == nil {
		articles = []store.Article{}
	}
	WriteOK(c.Writer, articles)
}

// GetArticle GET /api/articles/{slug} 公开详情。
func (s *Server) GetArticle(c *gin.Context) {
	a, err := s.st.GetPublishedArticleBySlug(c.Request.Context(), c.Param("slug"))
	if errors.Is(err, store.ErrNotFound) {
		WriteErr(c.Writer, http.StatusNotFound, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		WriteErr(c.Writer, http.StatusInternalServerError, http.StatusInternalServerError, "internal error")
		return
	}
	WriteOK(c.Writer, a)
}

// ListArticlesAdmin GET /api/admin/articles 全部状态列表。
func (s *Server) ListArticlesAdmin(c *gin.Context) {
	articles, err := s.st.ListArticlesAdmin(c.Request.Context())
	if err != nil {
		WriteErr(c.Writer, http.StatusInternalServerError, http.StatusInternalServerError, "internal error")
		return
	}
	if articles == nil {
		articles = []store.Article{}
	}
	WriteOK(c.Writer, articles)
}

// CreateArticle POST /api/admin/articles。
func (s *Server) CreateArticle(c *gin.Context) {
	var req articleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		WriteErr(c.Writer, http.StatusBadRequest, http.StatusBadRequest, "bad request")
		return
	}
	a := req.toArticle()
	if err := s.st.InsertArticle(c.Request.Context(), a); err != nil {
		writeStoreErr(c, err)
		return
	}
	WriteOK(c.Writer, a)
}

// UpdateArticle PUT /api/admin/articles/{id}。
func (s *Server) UpdateArticle(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		WriteErr(c.Writer, http.StatusBadRequest, http.StatusBadRequest, "bad request")
		return
	}
	var req articleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		WriteErr(c.Writer, http.StatusBadRequest, http.StatusBadRequest, "bad request")
		return
	}
	a := req.toArticle()
	a.ID = id
	if err := s.st.UpdateArticle(c.Request.Context(), a); err != nil {
		writeStoreErr(c, err)
		return
	}
	WriteOK(c.Writer, a)
}

// DeleteArticle DELETE /api/admin/articles/{id}。
func (s *Server) DeleteArticle(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		WriteErr(c.Writer, http.StatusBadRequest, http.StatusBadRequest, "bad request")
		return
	}
	if err := s.st.DeleteArticle(c.Request.Context(), id); err != nil {
		writeStoreErr(c, err)
		return
	}
	WriteOK(c.Writer, map[string]bool{"ok": true})
}

// writeStoreErr 把 store 的哨兵错误映射成 HTTP 状态。
func writeStoreErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		WriteErr(c.Writer, http.StatusNotFound, http.StatusNotFound, "not found")
	case errors.Is(err, store.ErrValidation):
		WriteErr(c.Writer, http.StatusBadRequest, http.StatusBadRequest, "bad request")
	default:
		WriteErr(c.Writer, http.StatusInternalServerError, http.StatusInternalServerError, "internal error")
	}
}
