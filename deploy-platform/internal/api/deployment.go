package api

import (
	"errors"
	"net/http"
	"strconv"

	"gantry/deploy-platform/internal/repo"
	"gantry/deploy-platform/internal/service"

	"github.com/gin-gonic/gin"
)

// createDeployment 将发布请求交给 Service，并把领域错误映射为 HTTP 状态码
func (h *Handler) createDeployment(c *gin.Context) {
	var req struct {
		AppID     int64 `json:"app_id" binding:"required"`
		VersionID int64 `json:"version_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数校验失败: "+err.Error())
		return
	}

	dep, err := h.deployments.CreateDeployment(c.Request.Context(), req.AppID, req.VersionID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAppNotFound), errors.Is(err, service.ErrVersionNotFound):
			respondError(c, http.StatusNotFound, err.Error())
		case errors.Is(err, service.ErrVersionAppMismatch):
			respondError(c, http.StatusBadRequest, err.Error())
		case errors.Is(err, repo.ErrActiveDeploy):
			respondError(c, http.StatusConflict, err.Error())
		case errors.Is(err, service.ErrQueueState):
			respondError(c, http.StatusInternalServerError, service.ErrQueueState.Error())
		default:
			respondError(c, http.StatusInternalServerError, err.Error())
		}
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"id":     dep.ID,
		"status": dep.Status,
	})
}

// getDeployment 解析任务主键并返回当前持久化状态
func (h *Handler) getDeployment(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "无效的任务 ID")
		return
	}
	dep, err := h.repo.GetDeployment(c.Request.Context(), id)
	if err != nil {
		respondError(c, http.StatusNotFound, "任务不存在")
		return
	}
	c.JSON(http.StatusOK, dep)
}

// listEvents 按时间顺序返回指定任务的审计事件
func (h *Handler) listEvents(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "无效的任务 ID")
		return
	}
	events, err := h.repo.ListEvents(c.Request.Context(), id)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, events)
}

// listDeployments 按应用和状态筛选并分页返回发布任务
func (h *Handler) listDeployments(c *gin.Context) {
	page, pageSize, err := parsePagination(c)
	if err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	filter := repo.DeploymentFilter{
		Status:   c.Query("status"),
		Page:     page,
		PageSize: pageSize,
	}
	if raw := c.Query("app_id"); raw != "" {
		filter.AppID, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || filter.AppID < 1 {
			respondError(c, http.StatusBadRequest, "无效的应用 ID")
			return
		}
	}
	deployments, total, err := h.repo.ListDeployments(c.Request.Context(), filter)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items":     deployments,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}
