package controller

import (
	"errors"
	"strings"
	"time"

	"ai-gateway/model"
	"ai-gateway/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AnnouncementController struct {
	db *gorm.DB
}

func NewAnnouncementController(db *gorm.DB) *AnnouncementController {
	return &AnnouncementController{db: db}
}

type announcementRequest struct {
	Title       string `json:"title" binding:"required,min=2,max=160"`
	Summary     string `json:"summary"`
	Content     string `json:"content" binding:"required,min=2"`
	LinkText    string `json:"link_text"`
	LinkURL     string `json:"link_url"`
	SortOrder   int    `json:"sort_order"`
	Pinned      bool   `json:"pinned"`
	Enabled     bool   `json:"enabled"`
	PublishedAt string `json:"published_at"`
}

func (a *AnnouncementController) PublicList(c *gin.Context) {
	var items []model.Announcement
	a.db.Where("enabled = ?", true).
		Order("pinned desc, published_at desc, sort_order asc, id desc").
		Limit(20).
		Find(&items)
	response.OK(c, items)
}

func (a *AnnouncementController) AdminList(c *gin.Context) {
	var items []model.Announcement
	a.db.Order("pinned desc, published_at desc, id desc").Find(&items)
	response.OK(c, items)
}

func (a *AnnouncementController) Create(c *gin.Context) {
	var req announcementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	publishedAt, err := parseAnnouncementTime(req.PublishedAt)
	if err != nil {
		response.Error(c, 400, "invalid published_at")
		return
	}
	if publishedAt == nil {
		now := time.Now()
		publishedAt = &now
	}
	item := model.Announcement{
		Title:       strings.TrimSpace(req.Title),
		Summary:     strings.TrimSpace(req.Summary),
		Content:     strings.TrimSpace(req.Content),
		LinkText:    strings.TrimSpace(req.LinkText),
		LinkURL:     strings.TrimSpace(req.LinkURL),
		SortOrder:   req.SortOrder,
		Pinned:      req.Pinned,
		Enabled:     req.Enabled,
		PublishedAt: publishedAt,
	}
	if err := a.db.Create(&item).Error; err != nil {
		response.Error(c, 500, "failed to create announcement")
		return
	}
	response.Created(c, item)
}

func (a *AnnouncementController) Update(c *gin.Context) {
	var req announcementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	publishedAt, err := parseAnnouncementTime(req.PublishedAt)
	if err != nil {
		response.Error(c, 400, "invalid published_at")
		return
	}
	updates := map[string]interface{}{
		"title":        strings.TrimSpace(req.Title),
		"summary":      strings.TrimSpace(req.Summary),
		"content":      strings.TrimSpace(req.Content),
		"link_text":    strings.TrimSpace(req.LinkText),
		"link_url":     strings.TrimSpace(req.LinkURL),
		"sort_order":   req.SortOrder,
		"pinned":       req.Pinned,
		"enabled":      req.Enabled,
		"published_at": publishedAt,
	}
	if err := a.db.Model(&model.Announcement{}).Where("id = ?", c.Param("id")).Updates(updates).Error; err != nil {
		response.Error(c, 500, "failed to update announcement")
		return
	}
	response.OK(c, nil)
}

func (a *AnnouncementController) Delete(c *gin.Context) {
	if err := a.db.Delete(&model.Announcement{}, c.Param("id")).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Error(c, 404, "announcement not found")
			return
		}
		response.Error(c, 500, "failed to delete announcement")
		return
	}
	response.OK(c, nil)
}

func parseAnnouncementTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	layouts := []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04", "2006-01-02"}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return &t, nil
		}
	}
	return nil, errors.New("invalid time")
}
