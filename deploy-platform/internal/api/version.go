package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// createVersion 校验父应用和请求体后创建应用内唯一的版本标签
func (h *Handler) createVersion(c *gin.Context) {
	appID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "无效的应用 ID")
		return
	}

	var req struct {
		Tag         string `json:"tag" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数校验失败: "+err.Error())
		return
	}
	if _, err := h.repo.GetApp(c.Request.Context(), appID); err != nil {
		respondError(c, http.StatusNotFound, "应用不存在")
		return
	}

	version, err := h.repo.CreateVersion(c.Request.Context(), appID, req.Tag, req.Description)
	if err != nil {
		respondError(c, http.StatusConflict, "版本 tag 重复: "+err.Error())
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"id": version.ID,
	})
}

// listVersions 返回指定应用的版本历史
func (h *Handler) listVersions(c *gin.Context) {
	appID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || appID < 1 {
		respondError(c, http.StatusBadRequest, "无效的应用 ID")
		return
	}
	if _, err := h.repo.GetApp(c.Request.Context(), appID); err != nil {
		respondError(c, http.StatusNotFound, "应用不存在")
		return
	}
	versions, err := h.repo.ListVersions(c.Request.Context(), appID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, versions)
}
