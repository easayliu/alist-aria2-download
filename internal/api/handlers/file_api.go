package handlers

import (
	"net/http"
	"time"

	"github.com/easayliu/alist-aria2-download/internal/application/services"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/alist"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/aria2"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/easayliu/alist-aria2-download/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ManualDownloadFiles 手动执行文件下载
// @Summary 手动执行文件下载
// @Description 手动执行指定时间范围内的文件下载，支持预览模式
// @Tags 文件管理
// @Accept json
// @Produce json
// @Param request body ManualDownloadRequest true "下载参数"
// @Success 200 {object} map[string]interface{} "下载结果"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /files/manual-download [post]
func ManualDownloadFiles(c *gin.Context) {
	var req ManualDownloadRequest

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

	// 计算时间范围
	var startTime, endTime time.Time
	
	if req.StartTime != "" && req.EndTime != "" {
		// 使用指定的时间范围
		timeRange, err := utils.ParseTimeRange(req.StartTime, req.EndTime)
		if err != nil {
			utils.ErrorWithStatus(c, http.StatusBadRequest, 400, "Invalid time format: "+err.Error())
			return
		}
		startTime, endTime = timeRange.Start, timeRange.End
	} else {
		// 使用 hours_ago 计算时间范围，如果没有指定则默认24小时
		if req.HoursAgo == 0 {
			req.HoursAgo = 24
		}
		timeRange := utils.CreateTimeRangeFromHours(req.HoursAgo)
		startTime, endTime = timeRange.Start, timeRange.End
	}

	// 获取指定时间范围内的文件
	files, err := fileService.GetFilesByTimeRange(req.Path, startTime, endTime, req.VideoOnly)
	if err != nil {
		utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to get files: "+err.Error())
		return
	}

	// 统计媒体类型
	var tvCount, movieCount, otherCount int
	var totalSize int64
	for _, file := range files {
		totalSize += file.Size
		switch file.MediaType {
		case "tv":
			tvCount++
		case "movie":
			movieCount++
		default:
			otherCount++
		}
	}

	// 构建响应数据
	response := gin.H{
		"path":        req.Path,
		"hours_ago":   req.HoursAgo,
		"video_only":  req.VideoOnly,
		"preview":     req.Preview,
		"start_time":  startTime.Format(time.RFC3339),
		"end_time":    endTime.Format(time.RFC3339),
		"total_files": len(files),
		"total_size":  totalSize,
		"media_stats": utils.BuildMediaStats(tvCount, movieCount, otherCount),
	}

	// 如果没有文件，直接返回
	if len(files) == 0 {
		response["message"] = "No files found in the specified time range"
		utils.Success(c, response)
		return
	}

	// 如果是预览模式，只返回文件信息，不进行下载
	if req.Preview {
		previewResults := make([]map[string]interface{}, 0, len(files))
		for _, file := range files {
			previewResults = append(previewResults, map[string]interface{}{
				"name":          file.Name,
				"path":          file.Path,
				"size":          file.Size,
				"media_type":    file.MediaType,
				"download_path": file.DownloadPath,
				"internal_url":  file.InternalURL,
				"modified_time": file.Modified,
			})
		}

		response["message"] = "Preview mode - no downloads initiated"
		response["files"] = previewResults
		utils.Success(c, response)
		return
	}

	// 执行下载
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
				"name":          file.Name,
				"path":          file.Path,
				"media_type":    file.MediaType,
				"download_path": file.DownloadPath,
				"status":        "failed",
				"error":         err.Error(),
			})
		} else {
			successCount++
			downloadResults = append(downloadResults, map[string]interface{}{
				"name":          file.Name,
				"path":          file.Path,
				"media_type":    file.MediaType,
				"download_path": file.DownloadPath,
				"status":        "success",
				"gid":           gid,
			})
		}
	}

	response["message"] = "Download tasks created"
	response["success_count"] = successCount
	response["fail_count"] = failCount
	response["results"] = downloadResults

	utils.Success(c, response)
}