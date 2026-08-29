package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"gantry/deploy-platform/internal/repo"
	"gantry/deploy-platform/internal/service"
)

// Handler 负责 HTTP 协议转换和路由注册
type Handler struct {
	repo        *repo.Repo
	deployments *service.DeploymentService
}

// New 组装 HTTP Handler，简单查询直接使用 Repo，发布创建交给 Service 编排
func New(r *repo.Repo, deployments *service.DeploymentService) *Handler {
	return &Handler{
		repo:        r,
		deployments: deployments,
	}
}

// Register 注册 v1 业务路由和不带版本的健康检查路由
func (h *Handler) Register(engine *gin.Engine) {
	v1 := engine.Group("/api/v1")
	v1.POST("/apps", h.createApp)
	v1.GET("/apps", h.listApps)
	v1.POST("/apps/:id/versions", h.createVersion)
	v1.GET("/apps/:id/versions", h.listVersions)
	v1.GET("/apps/:id", h.getApp)
	v1.POST("/deployments", h.createDeployment)
	v1.GET("/deployments", h.listDeployments)
	v1.GET("/deployments/:id", h.getDeployment)
	v1.GET("/deployments/:id/events", h.listEvents)
	engine.GET("/healthz", h.healthz)
}

// parsePagination 读取公共分页参数，页大小最大为 100
func parsePagination(c *gin.Context) (page, pageSize int, err error) {
	page, pageSize = 1, 20
	if raw := c.Query("page"); raw != "" {
		page, err = strconv.Atoi(raw)
		if err != nil || page < 1 {
			return 0, 0, fmt.Errorf("page 必须是正整数")
		}
	}
	if raw := c.Query("page_size"); raw != "" {
		pageSize, err = strconv.Atoi(raw)
		if err != nil || pageSize < 1 || pageSize > 100 {
			return 0, 0, fmt.Errorf("page_size 必须在 1–100 之间")
		}
	}
	return page, pageSize, nil
}

// healthz 返回进程存活状态，不探测外部依赖
func (h *Handler) healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

// respondError 将 HTTP 状态码和可读消息转换为统一错误响应
func respondError(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, gin.H{
		"code": statusCode,
		"msg":  message,
	})
}
