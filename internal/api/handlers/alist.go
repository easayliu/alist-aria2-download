package handlers

import (
	"net/http"
	"time"

	"github.com/easayliu/alist-aria2-download/internal/infrastructure/alist"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/easayliu/alist-aria2-download/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ListFilesRequest 获取文件列表请求参数
type ListFilesRequest struct {
	Path    string `form:"path" json:"path"`
	Page    int    `form:"page" json:"page"`
	PerPage int    `form:"per_page" json:"per_page"`
}

// ListFiles 获取Alist文件列表
// @Summary 获取文件列表
// @Description 获取Alist中指定路径的文件列表，需要先调用登录接口。如果不传path参数，将使用配置文件中的默认路径
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
func ListFiles(c *gin.Context) {
	var req ListFilesRequest
	
	// 绑定查询参数
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.ErrorWithStatus(c, http.StatusBadRequest, 400, "Invalid request parameters: "+err.Error())
		return
	}

	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to load config")
		return
	}

	// 设置默认值
	if req.Path == "" {
		// 使用配置文件中的默认路径
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

	// 创建Alist客户端
	client := alist.NewClient(cfg.Alist.BaseURL, cfg.Alist.Username, cfg.Alist.Password)
	
	// 获取文件列表
	fileList, err := client.ListFiles(req.Path, req.Page, req.PerPage)
	if err != nil {
		utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to get file list: "+err.Error())
		return
	}

	// 简化响应数据
	var simplifiedFiles []alist.SimplifiedFileItem
	for _, file := range fileList.Data.Content {
		// 解析时间
		modTime, _ := time.Parse(time.RFC3339, file.Modified)
		
		simplifiedFiles = append(simplifiedFiles, alist.SimplifiedFileItem{
			Name:     file.Name,
			Path:     file.Path,
			Size:     file.Size,
			IsDir:    file.IsDir,
			Modified: modTime,
			Sign:     file.Sign,
		})
	}

	// 返回成功响应
	utils.Success(c, gin.H{
		"files":    simplifiedFiles,
		"total":    fileList.Data.Total,
		"page":     req.Page,
		"per_page": req.PerPage,
		"path":     req.Path,
		"provider": fileList.Data.Provider,
	})
}

// GetFileInfoRequest 获取文件信息请求参数
type GetFileInfoRequest struct {
	Path string `form:"path" json:"path" binding:"required"`
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
func GetFileInfo(c *gin.Context) {
	var req GetFileInfoRequest
	
	// 绑定查询参数
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.ErrorWithStatus(c, http.StatusBadRequest, 400, "Invalid request parameters: "+err.Error())
		return
	}

	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to load config")
		return
	}

	// 创建Alist客户端
	client := alist.NewClient(cfg.Alist.BaseURL, cfg.Alist.Username, cfg.Alist.Password)
	
	// 获取文件信息
	fileInfo, err := client.GetFileInfo(req.Path)
	if err != nil {
		utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to get file info: "+err.Error())
		return
	}

	// 解析时间
	modTime, _ := time.Parse(time.RFC3339, fileInfo.Data.Modified)
	createTime, _ := time.Parse(time.RFC3339, fileInfo.Data.Created)

	// 返回成功响应
	utils.Success(c, gin.H{
		"name":      fileInfo.Data.Name,
		"path":      req.Path,
		"size":      fileInfo.Data.Size,
		"is_dir":    fileInfo.Data.IsDir,
		"modified":  modTime,
		"created":   createTime,
		"sign":      fileInfo.Data.Sign,
		"thumb":     fileInfo.Data.Thumb,
		"type":      fileInfo.Data.Type,
		"raw_url":   fileInfo.Data.RawURL,
		"provider":  fileInfo.Data.Provider,
	})
}

// AlistLogin 调用/api/auth/login获取token
// @Summary Alist登录
// @Description 使用配置文件中的用户名密码登录Alist服务
// @Tags Alist管理
// @Produce json
// @Success 200 {object} map[string]interface{} "登录成功"
// @Failure 401 {object} map[string]interface{} "登录失败"
// @Router /alist/login [post]
func AlistLogin(c *gin.Context) {
	cfg, err := config.LoadConfig()
	if err != nil {
		utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to load config")
		return
	}

	client := alist.NewClient(cfg.Alist.BaseURL, cfg.Alist.Username, cfg.Alist.Password)
	
	if err := client.Login(); err != nil {
		utils.ErrorWithStatus(c, http.StatusUnauthorized, 401, "Failed to login to Alist: "+err.Error())
		return
	}

	utils.Success(c, gin.H{
		"message": "Login successful",
	})
}