// Package file provides handlers for file browsing and management operations.
// It handles file listing, downloading, renaming, and deletion via Telegram.
package file

import (
	"context"
	"path/filepath"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
)

// Handler 文件浏览处理器
type Handler struct {
	deps FileDeps
}

// NewHandler 创建文件处理器
func NewHandler(deps FileDeps) *Handler {
	return &Handler{
		deps: deps,
	}
}

// ================================
// 辅助方法
// ================================

// BuildFullPath 构建完整路径
func (h *Handler) BuildFullPath(file contracts.FileResponse, basePath string) string {
	if file.Path != "" {
		return file.Path
	}
	if basePath == "/" {
		return "/" + file.Name
	}
	return basePath + "/" + file.Name
}

// ListFilesSimple 简单列出文件
func (h *Handler) ListFilesSimple(path string, page, perPage int) ([]contracts.FileResponse, error) {
	req := contracts.FileListRequest{
		Path:     path,
		Page:     page,
		PageSize: perPage,
	}

	ctx := context.Background()
	resp, err := h.deps.GetFileService().ListFiles(ctx, req)
	if err != nil {
		return nil, err
	}

	// 合并文件和目录
	var allItems []contracts.FileResponse
	allItems = append(allItems, resp.Directories...)
	allItems = append(allItems, resp.Files...)

	return allItems, nil
}

// GetFileDownloadURL 获取文件下载 URL
func (h *Handler) GetFileDownloadURL(path, fileName string) string {
	fullPath := path + "/" + fileName
	if path == "/" {
		fullPath = "/" + fileName
	}

	ctx := context.Background()
	fileInfo, err := h.deps.GetFileService().GetFileInfo(ctx, fullPath)
	if err != nil {
		// 获取失败时回退到直接构建 URL
		return h.deps.GetConfig().Alist.BaseURL + "/d" + fullPath
	}

	return fileInfo.InternalURL
}

// GetParentPath 获取父目录路径
func (h *Handler) GetParentPath(path string) string {
	if path == "/" {
		return "/"
	}
	parentPath := filepath.Dir(path)
	if parentPath == "." {
		return "/"
	}
	return parentPath
}

// ================================
// 兼容类型定义
// ================================

// DirectoryDownloadStats 目录下载统计
type DirectoryDownloadStats struct {
	TotalFiles   int
	VideoFiles   int
	TotalSize    int64
	MovieCount   int
	TVCount      int
	OtherCount   int
	TotalSizeStr string
}

// DirectoryDownloadResult 目录下载结果
type DirectoryDownloadResult struct {
	Stats        DirectoryDownloadStats
	SuccessCount int
	FailedCount  int
	FailedFiles  []string
}
