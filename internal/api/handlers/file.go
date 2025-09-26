package handlers

import (
	"net/http"

	"github.com/easayliu/alist-aria2-download/internal/application/services"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/alist"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/aria2"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/easayliu/alist-aria2-download/pkg/utils"
	"github.com/gin-gonic/gin"
)

// GetYesterdayFilesRequest 获取昨天文件请求参数
type GetYesterdayFilesRequest struct {
	Path string `form:"path" json:"path"`
}

// GetYesterdayFiles 批量获取昨天的文件信息
// @Summary 获取昨天的文件
// @Description 批量获取昨天修改的文件信息，并将raw_url中的fcalist-public替换为fcalist-internal
// @Tags 文件管理
// @Accept json
// @Produce json
// @Param path query string false "搜索路径（留空使用配置的默认路径）"
// @Success 200 {object} map[string]interface{} "昨天的文件列表"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /files/yesterday [get]
func GetYesterdayFiles(c *gin.Context) {
	var req GetYesterdayFilesRequest

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

	// 设置默认路径
	if req.Path == "" {
		req.Path = cfg.Alist.DefaultPath
		if req.Path == "" {
			req.Path = "/"
		}
	}

	// 创建Alist客户端
	alistClient := alist.NewClient(cfg.Alist.BaseURL, cfg.Alist.Username, cfg.Alist.Password)

	// 创建文件服务
	fileService := services.NewFileService(alistClient)

	// 获取昨天的文件
	yesterdayFiles, err := fileService.GetYesterdayFiles(req.Path)
	if err != nil {
		utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to get yesterday files: "+err.Error())
		return
	}

	// 统计信息
	var totalSize int64
	var tvCount, movieCount, otherCount int
	internalURLs := make([]string, 0, len(yesterdayFiles))

	for _, file := range yesterdayFiles {
		totalSize += file.Size
		internalURLs = append(internalURLs, file.InternalURL)

		// 统计媒体类型
		switch file.MediaType {
		case "tv":
			tvCount++
		case "movie":
			movieCount++
		default:
			otherCount++
		}
	}

	// 返回成功响应
	utils.Success(c, gin.H{
		"files":         yesterdayFiles,
		"count":         len(yesterdayFiles),
		"total_size":    totalSize,
		"internal_urls": internalURLs,
		"search_path":   req.Path,
		"date":          "yesterday",
		"media_stats": gin.H{
			"tv":    tvCount,
			"movie": movieCount,
			"other": otherCount,
		},
	})
}

// DownloadPathRequest 下载路径请求参数
type DownloadPathRequest struct {
	Path      string `json:"path" binding:"required"`
	Recursive bool   `json:"recursive"`
	Preview   bool   `json:"preview"` // 预览模式，只生成路径不下载
}

// DownloadFilesFromPath 从指定路径获取并下载文件
// @Summary 从指定路径下载文件
// @Description 获取指定路径下的所有文件并添加到Aria2下载队列，支持递归下载子目录，支持预览模式
// @Tags 文件管理
// @Accept json
// @Produce json
// @Param request body DownloadPathRequest true "下载路径请求"
// @Success 200 {object} map[string]interface{} "下载任务创建结果或预览信息"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /files/download [post]
func DownloadFilesFromPath(c *gin.Context) {
	var req DownloadPathRequest

	// 绑定请求参数
	if err := c.ShouldBindJSON(&req); err != nil {
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
	alistClient := alist.NewClient(cfg.Alist.BaseURL, cfg.Alist.Username, cfg.Alist.Password)

	// 创建文件服务
	fileService := services.NewFileService(alistClient)

	// 获取指定路径的文件
	files, err := fileService.GetFilesFromPath(req.Path, req.Recursive)
	if err != nil {
		utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to get files: "+err.Error())
		return
	}

	if len(files) == 0 {
		utils.Success(c, gin.H{
			"message": "No files found in the specified path",
			"count":   0,
			"path":    req.Path,
		})
		return
	}

	// 统计媒体类型
	var tvCount, movieCount, otherCount int
	for _, file := range files {
		switch file.MediaType {
		case "tv":
			tvCount++
		case "movie":
			movieCount++
		default:
			otherCount++
		}
	}

	// 如果是预览模式，只返回路径信息，不进行下载
	if req.Preview {
		previewResults := make([]map[string]interface{}, 0, len(files))
		for _, file := range files {
			previewResults = append(previewResults, map[string]interface{}{
				"file":          file.Name,
				"source_path":   file.Path,
				"size":          file.Size,
				"media_type":    file.MediaType,
				"download_path": file.DownloadPath,
				"download_file": file.DownloadPath + "/" + file.Name,
				"internal_url":  file.InternalURL,
			})
		}

		utils.Success(c, gin.H{
			"message":     "Preview mode - no downloads initiated",
			"mode":        "preview",
			"source_path": req.Path,
			"recursive":   req.Recursive,
			"total":       len(files),
			"media_stats": gin.H{
				"tv":    tvCount,
				"movie": movieCount,
				"other": otherCount,
			},
			"files": previewResults,
		})
		return
	}

	// 创建Aria2客户端
	aria2Client := aria2.NewClient(cfg.Aria2.RpcURL, cfg.Aria2.Token)

	// 批量添加下载任务
	var successCount, failCount int
	downloadResults := make([]map[string]interface{}, 0, len(files))

	for _, file := range files {
		// 设置下载选项
		options := map[string]interface{}{
			"dir": file.DownloadPath,
			"out": file.Name,
		}

		// 添加下载任务（使用内部URL）
		gid, err := aria2Client.AddURI(file.InternalURL, options)
		if err != nil {
			failCount++
			downloadResults = append(downloadResults, map[string]interface{}{
				"file":          file.Name,
				"path":          file.Path,
				"media_type":    file.MediaType,
				"download_path": file.DownloadPath,
				"status":        "failed",
				"error":         err.Error(),
			})
		} else {
			successCount++
			downloadResults = append(downloadResults, map[string]interface{}{
				"file":          file.Name,
				"path":          file.Path,
				"media_type":    file.MediaType,
				"download_path": file.DownloadPath,
				"status":        "success",
				"gid":           gid,
			})
		}
	}

	// 返回结果
	utils.Success(c, gin.H{
		"message":       "Download tasks created",
		"mode":          "download",
		"source_path":   req.Path,
		"recursive":     req.Recursive,
		"total":         len(files),
		"success_count": successCount,
		"fail_count":    failCount,
		"media_stats": gin.H{
			"tv":    tvCount,
			"movie": movieCount,
			"other": otherCount,
		},
		"results": downloadResults,
	})
}

// DownloadYesterdayFilesRequest 下载昨天文件请求参数
type DownloadYesterdayFilesRequest struct {
	Path    string `form:"path" json:"path"`
	Preview bool   `form:"preview" json:"preview"` // 预览模式
}

// FileListRequest 列出文件请求参数
type FileListRequest struct {
	Path      string `json:"path"` // 路径，为空时使用默认路径
	Page      int    `json:"page"`
	PerPage   int    `json:"per_page"`
	VideoOnly bool   `json:"video_only"` // 是否只显示视频文件
}

// ListFilesHandler 列出指定路径的文件
// @Summary 列出指定路径的文件
// @Description 获取指定路径下的文件列表，支持分页和视频文件过滤
// @Tags 文件管理
// @Accept json
// @Produce json
// @Param request body FileListRequest true "列出文件请求"
// @Success 200 {object} map[string]interface{} "文件列表"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /files/list [post]
func ListFilesHandler(c *gin.Context) {
	var req FileListRequest

	// 绑定请求参数
	if err := c.ShouldBindJSON(&req); err != nil {
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
		req.Path = cfg.Alist.DefaultPath
		if req.Path == "" {
			req.Path = "/"
		}
	}
	if req.Page == 0 {
		req.Page = 1
	}
	if req.PerPage == 0 {
		req.PerPage = 100
	}

	// 创建Alist客户端
	alistClient := alist.NewClient(cfg.Alist.BaseURL, cfg.Alist.Username, cfg.Alist.Password)

	// 获取文件列表
	fileList, err := alistClient.ListFiles(req.Path, req.Page, req.PerPage)
	if err != nil {
		utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to list files: "+err.Error())
		return
	}

	// 如果需要过滤视频文件
	if req.VideoOnly {
		fileService := services.NewFileService(alistClient)
		filteredContent := make([]alist.FileItem, 0)

		for _, file := range fileList.Data.Content {
			// 如果是目录或者是视频文件，则包含
			if file.IsDir || fileService.IsVideoFile(file.Name) {
				filteredContent = append(filteredContent, file)
			}
		}

		fileList.Data.Content = filteredContent
	}

	// 统计信息
	videoCount := 0
	dirCount := 0
	otherCount := 0

	fileService := services.NewFileService(alistClient)
	for _, file := range fileList.Data.Content {
		if file.IsDir {
			dirCount++
		} else if fileService.IsVideoFile(file.Name) {
			videoCount++
		} else {
			otherCount++
		}
	}

	// 返回结果
	utils.Success(c, gin.H{
		"path":       req.Path,
		"page":       req.Page,
		"per_page":   req.PerPage,
		"total":      len(fileList.Data.Content),
		"video_only": req.VideoOnly,
		"files":      fileList.Data.Content,
		"stats": gin.H{
			"videos":      videoCount,
			"directories": dirCount,
			"others":      otherCount,
		},
	})
}

// DownloadYesterdayFiles 批量下载昨天的文件
// @Summary 批量下载昨天文件
// @Description 将昨天修改的文件批量添加到Aria2下载队列，使用内部URL，支持预览模式
// @Tags 文件管理
// @Accept json
// @Produce json
// @Param path query string false "搜索路径（留空使用配置的默认路径）"
// @Param preview query bool false "预览模式，只生成路径不下载"
// @Success 200 {object} map[string]interface{} "下载任务创建结果或预览信息"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /files/yesterday/download [post]
func DownloadYesterdayFiles(c *gin.Context) {
	var req DownloadYesterdayFilesRequest

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

	// 设置默认路径
	if req.Path == "" {
		req.Path = cfg.Alist.DefaultPath
		if req.Path == "" {
			req.Path = "/"
		}
	}

	// 创建Alist客户端
	alistClient := alist.NewClient(cfg.Alist.BaseURL, cfg.Alist.Username, cfg.Alist.Password)

	// 创建文件服务
	fileService := services.NewFileService(alistClient)

	// 获取昨天的文件
	yesterdayFiles, err := fileService.GetYesterdayFiles(req.Path)
	if err != nil {
		utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to get yesterday files: "+err.Error())
		return
	}

	if len(yesterdayFiles) == 0 {
		utils.Success(c, gin.H{
			"message": "No files found for yesterday",
			"count":   0,
		})
		return
	}

	// 统计媒体类型
	var tvCount, movieCount, otherCount int
	for _, file := range yesterdayFiles {
		switch file.MediaType {
		case "tv":
			tvCount++
		case "movie":
			movieCount++
		default:
			otherCount++
		}
	}

	// 如果是预览模式，只返回路径信息
	if req.Preview {
		previewResults := make([]map[string]interface{}, 0, len(yesterdayFiles))
		for _, file := range yesterdayFiles {
			previewResults = append(previewResults, map[string]interface{}{
				"file":          file.Name,
				"source_path":   file.Path,
				"size":          file.Size,
				"media_type":    file.MediaType,
				"download_path": file.DownloadPath,
				"download_file": file.DownloadPath + "/" + file.Name,
				"internal_url":  file.InternalURL,
				"modified":      file.Modified,
			})
		}

		utils.Success(c, gin.H{
			"message":     "Preview mode - no downloads initiated",
			"mode":        "preview",
			"search_path": req.Path,
			"date":        "yesterday",
			"total":       len(yesterdayFiles),
			"media_stats": gin.H{
				"tv":    tvCount,
				"movie": movieCount,
				"other": otherCount,
			},
			"files": previewResults,
		})
		return
	}

	// 创建Aria2客户端
	aria2Client := aria2.NewClient(cfg.Aria2.RpcURL, cfg.Aria2.Token)

	// 批量添加下载任务
	var successCount, failCount int
	downloadResults := make([]map[string]interface{}, 0, len(yesterdayFiles))

	for _, file := range yesterdayFiles {
		// 设置下载选项，使用文件服务判断的下载路径
		options := map[string]interface{}{
			"dir": file.DownloadPath,
			"out": file.Name,
		}

		// 添加下载任务（使用内部URL）
		gid, err := aria2Client.AddURI(file.InternalURL, options)
		if err != nil {
			failCount++
			downloadResults = append(downloadResults, map[string]interface{}{
				"file":       file.Name,
				"media_type": file.MediaType,
				"path":       file.DownloadPath,
				"status":     "failed",
				"error":      err.Error(),
			})
		} else {
			successCount++
			downloadResults = append(downloadResults, map[string]interface{}{
				"file":       file.Name,
				"media_type": file.MediaType,
				"path":       file.DownloadPath,
				"status":     "success",
				"gid":        gid,
				"url":        file.InternalURL,
			})
		}
	}

	// 返回结果
	utils.Success(c, gin.H{
		"message":       "Batch download tasks created",
		"mode":          "download",
		"total":         len(yesterdayFiles),
		"success_count": successCount,
		"fail_count":    failCount,
		"media_stats": gin.H{
			"tv":    tvCount,
			"movie": movieCount,
			"other": otherCount,
		},
		"results": downloadResults,
	})
}
