package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// createApp 校验应用配置、补齐健康检查默认值并返回新应用 ID
func (h *Handler) createApp(c *gin.Context) {
	var req struct {
		Name            string `json:"name" binding:"required"`
		ImageName       string `json:"image_name" binding:"required"`
		HealthcheckPath string `json:"healthcheck_path"`
		HealthTimeout   int    `json:"healthcheck_timeout_sec"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数校验失败: "+err.Error())
		return
	}
	if req.HealthcheckPath == "" {
		req.HealthcheckPath = "/healthz"
	}
	if req.HealthTimeout == 0 {
		req.HealthTimeout = 60
	}
	if !strings.HasPrefix(req.HealthcheckPath, "/") || req.HealthTimeout < 1 || req.HealthTimeout > 300 {
		respondError(c, http.StatusBadRequest, "健康检查路径必须以 / 开头，超时范围为 1–300 秒")
		return
	}

	app, err := h.repo.CreateApp(c.Request.Context(), req.Name, req.ImageName, req.HealthcheckPath, req.HealthTimeout)
	if err != nil {
		respondError(c, http.StatusConflict, "应用名重复: "+err.Error())
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"id": app.ID,
	})
}

// getApp 解析路径主键并返回应用详情
func (h *Handler) getApp(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "无效的应用 ID")
		return
	}
	app, err := h.repo.GetApp(c.Request.Context(), id)
	if err != nil {
		respondError(c, http.StatusNotFound, "应用不存在")
		return
	}
	c.JSON(http.StatusOK, app)
}

// listApps 分页返回管理页面需要的应用列表
func (h *Handler) listApps(c *gin.Context) {
	page, pageSize, err := parsePagination(c)
	if err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	apps, total, err := h.repo.ListApps(c.Request.Context(), page, pageSize)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items":     apps,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}
