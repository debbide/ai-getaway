package controller

import (
	"errors"
	"strconv"
	"strings"

	"ai-gateway/model"
	"ai-gateway/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type DocsController struct {
	db *gorm.DB
}

func NewDocsController(db *gorm.DB) *DocsController {
	return &DocsController{db: db}
}

type docPageRequest struct {
	Title       string `json:"title" binding:"required,min=2,max=128"`
	Slug        string `json:"slug" binding:"required,min=2,max=128"`
	GroupName   string `json:"group_name"`
	Description string `json:"description"`
	Content     string `json:"content"`
	SortOrder   int    `json:"sort_order"`
	Enabled     bool   `json:"enabled"`
}

func (d *DocsController) PublicList(c *gin.Context) {
	var docs []model.DocPage
	d.db.Where("enabled = ?", true).Order("sort_order asc, id asc").Find(&docs)
	response.OK(c, docs)
}

func (d *DocsController) AdminList(c *gin.Context) {
	page, pageSize := 1, 10
	if value, err := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("page", "1"))); err == nil && value > 0 {
		page = value
	}
	if value, err := strconv.Atoi(strings.TrimSpace(c.Query("page_size"))); err == nil && value > 0 {
		pageSize = value
	}
	if pageSize > 200 {
		pageSize = 200
	}

	query := d.db.Model(&model.DocPage{})
	if keyword := strings.TrimSpace(c.Query("q")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("title LIKE ? OR slug LIKE ? OR group_name LIKE ? OR description LIKE ? OR CAST(id AS CHAR) LIKE ?", like, like, like, like, like)
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		query = query.Where("enabled = ?", status == "enabled")
	}
	if group := strings.TrimSpace(c.Query("group_name")); group != "" {
		query = query.Where("group_name LIKE ?", "%"+group+"%")
	}

	var total int64
	query.Count(&total)
	var docs []model.DocPage
	query.Order("sort_order asc, id asc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&docs)
	response.OK(c, gin.H{
		"items":     docs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (d *DocsController) Create(c *gin.Context) {
	var req docPageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	doc := model.DocPage{
		Title:       strings.TrimSpace(req.Title),
		Slug:        normalizeDocSlug(req.Slug),
		GroupName:   strings.TrimSpace(req.GroupName),
		Description: strings.TrimSpace(req.Description),
		Content:     req.Content,
		SortOrder:   req.SortOrder,
		Enabled:     req.Enabled,
	}
	if err := d.db.Create(&doc).Error; err != nil {
		response.Error(c, 500, "failed to create doc page")
		return
	}
	response.Created(c, doc)
}

func (d *DocsController) Update(c *gin.Context) {
	var req docPageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	updates := map[string]interface{}{
		"title":       strings.TrimSpace(req.Title),
		"slug":        normalizeDocSlug(req.Slug),
		"group_name":  strings.TrimSpace(req.GroupName),
		"description": strings.TrimSpace(req.Description),
		"content":     req.Content,
		"sort_order":  req.SortOrder,
		"enabled":     req.Enabled,
	}
	result := d.db.Model(&model.DocPage{}).Where("id = ?", c.Param("id")).Updates(updates)
	if result.Error != nil {
		response.Error(c, 500, "failed to update doc page")
		return
	}
	if result.RowsAffected == 0 {
		response.Error(c, 404, "doc page not found")
		return
	}
	response.OK(c, nil)
}

func (d *DocsController) Delete(c *gin.Context) {
	result := d.db.Delete(&model.DocPage{}, c.Param("id"))
	if result.Error != nil {
		response.Error(c, 500, "failed to delete doc page")
		return
	}
	if result.RowsAffected == 0 {
		response.Error(c, 404, "doc page not found")
		return
	}
	response.OK(c, nil)
}

func (d *DocsController) PublicBySlug(c *gin.Context) {
	var doc model.DocPage
	err := d.db.Where("slug = ? AND enabled = ?", c.Param("slug"), true).First(&doc).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Error(c, 404, "doc page not found")
			return
		}
		response.Error(c, 500, "failed to load doc page")
		return
	}
	response.OK(c, doc)
}

func normalizeDocSlug(value string) string {
	slug := strings.ToLower(strings.TrimSpace(value))
	slug = strings.ReplaceAll(slug, " ", "-")
	return slug
}
