package handlers

import (
	"net/http"

	"github.com/easayliu/alist-aria2-download/internal/application/services"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/alist"
	timeutil "github.com/easayliu/alist-aria2-download/pkg/utils/time"
	httputil "github.com/easayliu/alist-aria2-download/pkg/utils/http"
	"github.com/gin-gonic/gin"
)

type AlistHandler struct {
	container *services.ServiceContainer
}

func NewAlistHandler(container *services.ServiceContainer) *AlistHandler {
	return &AlistHandler{
		container: container,
	}
}

// ListFiles 获取Alist文件列表
// @Summary 获取文件列表
// @Description 获取Alist中指定路径的文件列表
// @Tags Alist管理
// @Accept json
// @Produce json
// @Param path query string false "文件路径（留空使用配置的默认路径）"
// @Param page query int false "页码" default(1)
// @Param per_page query int false "每页数量" default(20)
// @Success 200 {object} map[string]interface{} "文件列表"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /alist/files [get]
func (h *AlistHandler) ListFiles(c *gin.Context) {
	var req struct {
		Path    string `form:"path" json:"path"`
		Page    int    `form:"page" json:"page"`
		PerPage int    `form:"per_page" json:"per_page"`
	}

	if err := c.ShouldBindQuery(&req); err != nil {
		httputil.ErrorWithStatus(c, http.StatusBadRequest, 400, "Invalid request parameters: "+err.Error())
		return
	}

	cfg := h.container.GetConfig()

	if req.Path == "" {
		req.Path = cfg.Alist.DefaultPath
		if req.Path == "" {
			req.Path = "/"
		}
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PerPage <= 0 {
		req.PerPage = 20
	}

	client := alist.NewClient(cfg.Alist.BaseURL, cfg.Alist.Username, cfg.Alist.Password)
	fileList, err := client.ListFiles(req.Path, req.Page, req.PerPage)
	if err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to get file list: "+err.Error())
		return
	}

	var simplifiedFiles []alist.SimplifiedFileItem
	for _, file := range fileList.Data.Content {
		modTime := timeutil.ParseTimeOrZero(file.Modified)
		simplifiedFiles = append(simplifiedFiles, alist.SimplifiedFileItem{
			Name:     file.Name,
			Path:     file.Path,
			Size:     file.Size,
			IsDir:    file.IsDir,
			Modified: modTime,
			Sign:     file.Sign,
		})
	}

	httputil.Success(c, gin.H{
		"files":    simplifiedFiles,
		"total":    fileList.Data.Total,
		"page":     req.Page,
		"per_page": req.PerPage,
		"path":     req.Path,
		"provider": fileList.Data.Provider,
	})
}

// GetFileInfo 获取文件详细信息
// @Summary 获取文件信息
// @Description 获取Alist中指定路径文件的详细信息，包含下载链接
// @Tags Alist管理
// @Accept json
// @Produce json
// @Param path query string true "文件完整路径"
// @Success 200 {object} map[string]interface{} "文件详细信息"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /alist/file [get]
func (h *AlistHandler) GetFileInfo(c *gin.Context) {
	var req struct {
		Path string `form:"path" json:"path" binding:"required"`
	}

	if err := c.ShouldBindQuery(&req); err != nil {
		httputil.ErrorWithStatus(c, http.StatusBadRequest, 400, "Invalid request parameters: "+err.Error())
		return
	}

	cfg := h.container.GetConfig()
	client := alist.NewClient(cfg.Alist.BaseURL, cfg.Alist.Username, cfg.Alist.Password)

	fileInfo, err := client.GetFileInfo(req.Path)
	if err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to get file info: "+err.Error())
		return
	}

	modTime := timeutil.ParseTimeOrZero(fileInfo.Data.Modified)
	createTime := timeutil.ParseTimeOrZero(fileInfo.Data.Created)

	httputil.Success(c, gin.H{
		"name":     fileInfo.Data.Name,
		"path":     req.Path,
		"size":     fileInfo.Data.Size,
		"is_dir":   fileInfo.Data.IsDir,
		"modified": modTime,
		"created":  createTime,
		"sign":     fileInfo.Data.Sign,
		"thumb":    fileInfo.Data.Thumb,
		"type":     fileInfo.Data.Type,
		"raw_url":  fileInfo.Data.RawURL,
		"provider": fileInfo.Data.Provider,
	})
}

// Login Alist登录
// @Summary Alist登录
// @Description 使用配置文件中的用户名密码登录Alist服务
// @Tags Alist管理
// @Produce json
// @Success 200 {object} map[string]interface{} "登录成功"
// @Failure 401 {object} map[string]interface{} "登录失败"
// @Router /alist/login [post]
func (h *AlistHandler) Login(c *gin.Context) {
	cfg := h.container.GetConfig()
	client := alist.NewClient(cfg.Alist.BaseURL, cfg.Alist.Username, cfg.Alist.Password)

	client.ClearToken()

	_, err := client.ListFiles("/", 1, 1)
	if err != nil {
		httputil.ErrorWithStatus(c, http.StatusUnauthorized, 401, "Failed to connect to Alist: "+err.Error())
		return
	}

	hasToken, isValid, expiryTime := client.GetTokenStatus()

	httputil.Success(c, gin.H{
		"message":     "Connection successful",
		"has_token":   hasToken,
		"token_valid": isValid,
		"expires_at":  expiryTime.Format("2006-01-02 15:04:05"),
	})
}
