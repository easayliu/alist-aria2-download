package handlers

import (
	"net/http"
	"strconv"

	"github.com/easayliu/alist-aria2-download/internal/infrastructure/aria2"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/easayliu/alist-aria2-download/pkg/utils"
	"github.com/gin-gonic/gin"
)

// CreateDownloadRequest 创建下载请求
type CreateDownloadRequest struct {
	URL      string                 `json:"url" binding:"required"`
	Filename string                 `json:"filename,omitempty"`
	Dir      string                 `json:"dir,omitempty"`
	Options  map[string]interface{} `json:"options,omitempty"`
}

// CreateDownload 创建下载任务
// @Summary 创建下载任务
// @Description 创建新的Aria2下载任务
// @Tags 下载管理
// @Accept json
// @Produce json
// @Param request body CreateDownloadRequest true "下载请求参数"
// @Success 200 {object} map[string]interface{} "下载任务创建成功"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /downloads [post]
func CreateDownload(c *gin.Context) {
	var req CreateDownloadRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorWithStatus(c, http.StatusBadRequest, 400, "Invalid request: "+err.Error())
		return
	}

	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to load config")
		return
	}

	// 创建Aria2客户端
	aria2Client := aria2.NewClient(cfg.Aria2.RpcURL, cfg.Aria2.Token)

	// 设置下载选项
	options := req.Options
	if options == nil {
		options = make(map[string]interface{})
	}

	// 设置下载目录
	if req.Dir != "" {
		options["dir"] = req.Dir
	} else if cfg.Aria2.DownloadDir != "" {
		options["dir"] = cfg.Aria2.DownloadDir
	}

	// 设置文件名
	if req.Filename != "" {
		options["out"] = req.Filename
	}

	// 添加下载任务
	gid, err := aria2Client.AddURI(req.URL, options)
	if err != nil {
		utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to create download: "+err.Error())
		return
	}

	utils.Success(c, gin.H{
		"gid":     gid,
		"message": "Download created successfully",
		"url":     req.URL,
	})
}

// ListDownloads 获取下载列表
// @Summary 获取下载列表
// @Description 获取所有Aria2下载任务列表
// @Tags 下载管理
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /downloads [get]
func ListDownloads(c *gin.Context) {
	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to load config")
		return
	}

	// 创建Aria2客户端
	aria2Client := aria2.NewClient(cfg.Aria2.RpcURL, cfg.Aria2.Token)

	// 获取活动下载
	active, err := aria2Client.GetActive()
	if err != nil {
		utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to get active downloads: "+err.Error())
		return
	}

	// 获取等待下载
	waiting, err := aria2Client.GetWaiting(0, 100)
	if err != nil {
		utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to get waiting downloads: "+err.Error())
		return
	}

	// 获取已停止下载
	stopped, err := aria2Client.GetStopped(0, 100)
	if err != nil {
		utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to get stopped downloads: "+err.Error())
		return
	}

	// 获取全局统计
	globalStat, err := aria2Client.GetGlobalStat()
	if err != nil {
		globalStat = make(map[string]interface{})
	}

	utils.Success(c, gin.H{
		"active":      active,
		"waiting":     waiting,
		"stopped":     stopped,
		"global_stat": globalStat,
	})
}

// GetDownload 获取单个下载详情
// @Summary 获取下载详情
// @Description 根据GID获取单个下载任务详情
// @Tags 下载管理
// @Produce json
// @Param id path string true "下载任务GID"
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /downloads/{id} [get]
func GetDownload(c *gin.Context) {
	gid := c.Param("id")

	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to load config")
		return
	}

	// 创建Aria2客户端
	aria2Client := aria2.NewClient(cfg.Aria2.RpcURL, cfg.Aria2.Token)

	// 获取下载状态
	status, err := aria2Client.GetStatus(gid)
	if err != nil {
		utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to get download status: "+err.Error())
		return
	}

	// 转换数据
	totalLength, _ := strconv.ParseInt(status.TotalLength, 10, 64)
	completedLength, _ := strconv.ParseInt(status.CompletedLength, 10, 64)
	downloadSpeed, _ := strconv.ParseInt(status.DownloadSpeed, 10, 64)

	var progress float64
	if totalLength > 0 {
		progress = float64(completedLength) / float64(totalLength) * 100
	}

	utils.Success(c, gin.H{
		"gid":              status.GID,
		"status":           status.Status,
		"total_length":     totalLength,
		"completed_length": completedLength,
		"download_speed":   downloadSpeed,
		"progress":         progress,
		"files":            status.Files,
		"error_code":       status.ErrorCode,
		"error_message":    status.ErrorMessage,
	})
}

// DeleteDownload 删除下载任务
// @Summary 删除下载任务
// @Description 根据GID删除下载任务
// @Tags 下载管理
// @Produce json
// @Param id path string true "下载任务GID"
// @Success 200 {object} map[string]string
// @Failure 500 {object} map[string]interface{}
// @Router /downloads/{id} [delete]
func DeleteDownload(c *gin.Context) {
	gid := c.Param("id")

	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to load config")
		return
	}

	// 创建Aria2客户端
	aria2Client := aria2.NewClient(cfg.Aria2.RpcURL, cfg.Aria2.Token)

	// 删除下载
	if err := aria2Client.Remove(gid); err != nil {
		utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to delete download: "+err.Error())
		return
	}

	utils.Success(c, gin.H{
		"message": "Download " + gid + " deleted successfully",
	})
}

// PauseDownload 暂停下载
// @Summary 暂停下载
// @Description 暂停指定的下载任务
// @Tags 下载管理
// @Produce json
// @Param id path string true "下载任务GID"
// @Success 200 {object} map[string]string
// @Failure 500 {object} map[string]interface{}
// @Router /downloads/{id}/pause [post]
func PauseDownload(c *gin.Context) {
	gid := c.Param("id")

	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to load config")
		return
	}

	// 创建Aria2客户端
	aria2Client := aria2.NewClient(cfg.Aria2.RpcURL, cfg.Aria2.Token)

	// 暂停下载
	if err := aria2Client.Pause(gid); err != nil {
		utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to pause download: "+err.Error())
		return
	}

	utils.Success(c, gin.H{
		"message": "Download " + gid + " paused successfully",
	})
}

// ResumeDownload 恢复下载
// @Summary 恢复下载
// @Description 恢复指定的下载任务
// @Tags 下载管理
// @Produce json
// @Param id path string true "下载任务GID"
// @Success 200 {object} map[string]string
// @Failure 500 {object} map[string]interface{}
// @Router /downloads/{id}/resume [post]
func ResumeDownload(c *gin.Context) {
	gid := c.Param("id")

	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to load config")
		return
	}

	// 创建Aria2客户端
	aria2Client := aria2.NewClient(cfg.Aria2.RpcURL, cfg.Aria2.Token)

	// 恢复下载
	if err := aria2Client.Resume(gid); err != nil {
		utils.ErrorWithStatus(c, http.StatusInternalServerError, 500, "Failed to resume download: "+err.Error())
		return
	}

	utils.Success(c, gin.H{
		"message": "Download " + gid + " resumed successfully",
	})
}

