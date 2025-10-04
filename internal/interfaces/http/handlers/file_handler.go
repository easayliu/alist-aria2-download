package handlers

import (
	"net/http"

	"github.com/easayliu/alist-aria2-download/internal/application/services"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/alist"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/aria2"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/easayliu/alist-aria2-download/pkg/calculator"
	"github.com/easayliu/alist-aria2-download/pkg/executor"
	"github.com/easayliu/alist-aria2-download/pkg/formatter"
	pathutil "github.com/easayliu/alist-aria2-download/pkg/utils/path"
	httputil "github.com/easayliu/alist-aria2-download/pkg/utils/http"
	"github.com/gin-gonic/gin"
)

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

	// 获取昨天的文件
	yesterdayFiles, err := fileService.GetYesterdayFiles(req.Path)
	if err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to get yesterday files: "+err.Error())
		return
	}

	// 使用统一的统计计算器
	statsCalc := calculator.NewFileStatsCalculator()
	stats := statsCalc.CalculateFromYesterdayFileInfo(yesterdayFiles)

	// 提取内部URL
	internalURLs := make([]string, 0, len(yesterdayFiles))
	for _, file := range yesterdayFiles {
		internalURLs = append(internalURLs, file.InternalURL)
	}

	// 返回成功响应
	httputil.Success(c, gin.H{
		"files":         yesterdayFiles,
		"count":         stats.TotalCount,
		"total_size":    stats.TotalSize,
		"internal_urls": internalURLs,
		"search_path":   req.Path,
		"date":          "yesterday",
		"media_stats":   stats.BuildMediaStats(),
	})
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
		httputil.ErrorWithStatus(c, http.StatusBadRequest, 400, "Invalid request parameters: "+err.Error())
		return
	}

	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to load config")
		return
	}

	// 创建Alist客户端
	alistClient := alist.NewClient(cfg.Alist.BaseURL, cfg.Alist.Username, cfg.Alist.Password)

	// 创建文件服务
	fileService := services.NewFileService(alistClient)

	// 获取指定路径的文件
	files, err := fileService.GetFilesFromPath(req.Path, req.Recursive)
	if err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to get files: "+err.Error())
		return
	}

	if len(files) == 0 {
		httputil.Success(c, gin.H{
			"message": "No files found in the specified path",
			"count":   0,
			"path":    req.Path,
		})
		return
	}

	// 使用统一的统计计算器
	statsCalc := calculator.NewFileStatsCalculator()
	stats := statsCalc.CalculateFromFileInfo(files)

	// 如果是预览模式，使用统一的预览格式化器
	if req.Preview {
		previewFormatter := formatter.NewPreviewFormatter()
		response := previewFormatter.BuildDirectoryPreviewResponse(
			req.Path,
			files,
			req.Recursive,
			stats.BuildMediaStats(),
		)
		httputil.Success(c, response)
		return
	}

	// 创建Aria2客户端
	aria2Client := aria2.NewClient(cfg.Aria2.RpcURL, cfg.Aria2.Token)

	// 使用统一的批量下载执行器
	batchExecutor := executor.NewBatchDownloadExecutor(aria2Client, 5)
	downloadResult := batchExecutor.Execute(files)

	// 返回结果
	httputil.Success(c, gin.H{
		"message":       "Download tasks created",
		"mode":          "download",
		"source_path":   req.Path,
		"recursive":     req.Recursive,
		"total":         downloadResult.TotalCount,
		"success_count": downloadResult.SuccessCount,
		"fail_count":    downloadResult.FailCount,
		"media_stats":   stats.BuildMediaStats(),
		"results":       downloadResult.Results,
	})
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
		httputil.ErrorWithStatus(c, http.StatusBadRequest, 400, "Invalid request parameters: "+err.Error())
		return
	}

	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to load config")
		return
	}

	// 设置默认值
	req.Path = pathutil.ResolveDefaultPath(req.Path, cfg.Alist.DefaultPath)
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
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to list files: "+err.Error())
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
	httputil.Success(c, gin.H{
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

	// 获取昨天的文件
	yesterdayFiles, err := fileService.GetYesterdayFiles(req.Path)
	if err != nil {
		httputil.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to get yesterday files: "+err.Error())
		return
	}

	if len(yesterdayFiles) == 0 {
		httputil.Success(c, gin.H{
			"message": "No files found for yesterday",
			"count":   0,
		})
		return
	}

	// 使用统一的统计计算器
	statsCalc := calculator.NewFileStatsCalculator()
	stats := statsCalc.CalculateFromYesterdayFileInfo(yesterdayFiles)

	// 如果是预览模式，使用统一的预览格式化器
	if req.Preview {
		previewFormatter := formatter.NewPreviewFormatter()
		response := previewFormatter.BuildYesterdayPreviewResponse(
			req.Path,
			yesterdayFiles,
			stats.BuildMediaStats(),
		)
		httputil.Success(c, response)
		return
	}

	// 创建Aria2客户端并使用批量下载执行器
	aria2Client := aria2.NewClient(cfg.Aria2.RpcURL, cfg.Aria2.Token)
	batchExecutor := executor.NewBatchDownloadExecutor(aria2Client, 5)
	downloadResult := batchExecutor.Execute(convertYesterdayToFileInfo(yesterdayFiles))

	// 返回结果
	httputil.Success(c, gin.H{
		"message":       "Batch download tasks created",
		"mode":          "download",
		"total":         downloadResult.TotalCount,
		"success_count": downloadResult.SuccessCount,
		"fail_count":    downloadResult.FailCount,
		"media_stats":   stats.BuildMediaStats(),
		"results":       downloadResult.Results,
	})
}

// ========== 辅助函数 ==========

