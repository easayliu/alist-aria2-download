package handlers

import (
	"net/http"
	"time"

	"github.com/easayliu/alist-aria2-download/internal/application/services"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/alist"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/aria2"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/easayliu/alist-aria2-download/pkg/calculator"
	"github.com/easayliu/alist-aria2-download/pkg/executor"
	"github.com/easayliu/alist-aria2-download/pkg/formatter"
	timeutil "github.com/easayliu/alist-aria2-download/pkg/utils/time"
	pathutil "github.com/easayliu/alist-aria2-download/pkg/utils/path"
	httputil "github.com/easayliu/alist-aria2-download/pkg/utils/http"
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
		httputil.ErrorWithStatus(c, http.StatusBadRequest, 400, "Invalid request parameters: "+err.Error())
		return
	}

	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to load config")
		return
	}

	// 设置默认路径
	req.Path = pathutil.ResolveDefaultPath(req.Path, cfg.Alist.DefaultPath)

	// 创建Alist客户端
	alistClient := alist.NewClient(cfg.Alist.BaseURL, cfg.Alist.Username, cfg.Alist.Password)

	// 创建文件服务
	fileService := services.NewFileService(alistClient)

	// 计算时间范围
	var startTime, endTime time.Time
	
	if req.StartTime != "" && req.EndTime != "" {
		// 使用指定的时间范围
		timeRange, err := timeutil.ParseTimeRange(req.StartTime, req.EndTime)
		if err != nil {
			httputil.ErrorWithStatus(c, http.StatusBadRequest, 400, "Invalid time format: "+err.Error())
			return
		}
		startTime, endTime = timeRange.Start, timeRange.End
	} else {
		// 使用 hours_ago 计算时间范围，如果没有指定则默认24小时
		if req.HoursAgo == 0 {
			req.HoursAgo = 24
		}
		timeRange := timeutil.CreateTimeRangeFromHours(req.HoursAgo)
		startTime, endTime = timeRange.Start, timeRange.End
	}

	// 获取指定时间范围内的文件
	files, err := fileService.GetFilesByTimeRange(req.Path, startTime, endTime, req.VideoOnly)
	if err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to get files: "+err.Error())
		return
	}

	// 使用统一的统计计算器
	statsCalc := calculator.NewFileStatsCalculator()
	stats := statsCalc.CalculateFromYesterdayFileInfo(files)

	// 构建响应数据
	response := gin.H{
		"path":        req.Path,
		"hours_ago":   req.HoursAgo,
		"video_only":  req.VideoOnly,
		"preview":     req.Preview,
		"start_time":  startTime.Format(time.RFC3339),
		"end_time":    endTime.Format(time.RFC3339),
		"total_files": stats.TotalCount,
		"total_size":  stats.TotalSize,
		"media_stats": stats.BuildMediaStats(),
	}

	// 如果没有文件，直接返回
	if len(files) == 0 {
		response["message"] = "No files found in the specified time range"
		httputil.Success(c, response)
		return
	}

	// 如果是预览模式，使用统一的预览格式化器
	if req.Preview {
		previewFormatter := formatter.NewPreviewFormatter()
		previewResponse := previewFormatter.BuildTimeRangePreviewResponse(
			req.Path,
			files,
			startTime.Format(time.RFC3339),
			endTime.Format(time.RFC3339),
			stats.BuildMediaStats(),
		)
		// 合并基础响应字段
		for key, value := range response {
			previewResponse[key] = value
		}
		httputil.Success(c, previewResponse)
		return
	}

	// 执行下载 - 使用批量下载执行器
	// 需要先转换YesterdayFileInfo到FileInfo
	aria2Client := aria2.NewClient(cfg.Aria2.RpcURL, cfg.Aria2.Token)
	batchExecutor := executor.NewBatchDownloadExecutor(aria2Client, 5)
	downloadResult := batchExecutor.Execute(convertYesterdayToFileInfo(files))

	response["message"] = "Download tasks created"
	response["success_count"] = downloadResult.SuccessCount
	response["fail_count"] = downloadResult.FailCount
	response["results"] = downloadResult.Results

	httputil.Success(c, response)
}