package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/T-DWAG/blog_build/server/internal/store"
)

// projectReq 是管理台写项目的请求体。published 用 *bool：
// 省略 → nil → 默认 true（上线）；显式 false → 下线。
type projectReq struct {
	Name      string   `json:"name"`
	CoverURL  string   `json:"cover_url"`
	Summary   string   `json:"summary"`
	RepoURL   string   `json:"repo_url"`
	HomeURL   string   `json:"home_url"`
	DemoURL   string   `json:"demo_url"`
	DetailMD  string   `json:"detail_md"`
	Tags      []string `json:"tags"`
	SortOrder int      `json:"sort_order"`
	Published *bool    `json:"published"`
}

func (r projectReq) toProject() *store.Project {
	published := true // nil → 默认上线
	if r.Published != nil {
		published = *r.Published
	}
	return &store.Project{
		Name:      r.Name,
		CoverURL:  r.CoverURL,
		Summary:   r.Summary,
		RepoURL:   r.RepoURL,
		HomeURL:   r.HomeURL,
		DemoURL:   r.DemoURL,
		DetailMD:  r.DetailMD,
		Tags:      r.Tags,
		SortOrder: r.SortOrder,
		Published: published,
	}
}

// ListProjects GET /api/projects 公开列表，?tag=。
func (s *Server) ListProjects(c *gin.Context) {
	projects, err := s.st.ListPublishedProjects(c.Request.Context(), c.Query("tag"))
	if err != nil {
		WriteErr(c.Writer, http.StatusInternalServerError, http.StatusInternalServerError, "internal error")
		return
	}
	if projects == nil {
		projects = []store.Project{}
	}
	WriteOK(c.Writer, projects)
}

// ListProjectsAdmin GET /api/admin/projects 全部项目列表。
func (s *Server) ListProjectsAdmin(c *gin.Context) {
	projects, err := s.st.ListProjectsAdmin(c.Request.Context())
	if err != nil {
		WriteErr(c.Writer, http.StatusInternalServerError, http.StatusInternalServerError, "internal error")
		return
	}
	if projects == nil {
		projects = []store.Project{}
	}
	WriteOK(c.Writer, projects)
}

// CreateProject POST /api/admin/projects。
func (s *Server) CreateProject(c *gin.Context) {
	var req projectReq
	if err := c.ShouldBindJSON(&req); err != nil {
		WriteErr(c.Writer, http.StatusBadRequest, http.StatusBadRequest, "bad request")
		return
	}
	p := req.toProject()
	if err := s.st.InsertProject(c.Request.Context(), p); err != nil {
		writeStoreErr(c, err)
		return
	}
	WriteOK(c.Writer, p)
}

// UpdateProject PUT /api/admin/projects/{id}。
func (s *Server) UpdateProject(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		WriteErr(c.Writer, http.StatusBadRequest, http.StatusBadRequest, "bad request")
		return
	}
	var req projectReq
	if err := c.ShouldBindJSON(&req); err != nil {
		WriteErr(c.Writer, http.StatusBadRequest, http.StatusBadRequest, "bad request")
		return
	}
	p := req.toProject()
	p.ID = id
	if err := s.st.UpdateProject(c.Request.Context(), p); err != nil {
		writeStoreErr(c, err)
		return
	}
	WriteOK(c.Writer, p)
}

// DeleteProject DELETE /api/admin/projects/{id}。
func (s *Server) DeleteProject(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		WriteErr(c.Writer, http.StatusBadRequest, http.StatusBadRequest, "bad request")
		return
	}
	if err := s.st.DeleteProject(c.Request.Context(), id); err != nil {
		writeStoreErr(c, err)
		return
	}
	WriteOK(c.Writer, map[string]bool{"ok": true})
}
