package file

import (
	"context"
	"fmt"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
)

// DownloadFiles 批量下载文件
func (s *AppFileService) DownloadFiles(ctx context.Context, req contracts.BatchFileDownloadRequest) (*contracts.BatchDownloadResponse, error) {
	// 检查下载服务是否可用
	if s.downloadService == nil {
		return nil, fmt.Errorf("download service not available")
	}

	var downloadRequests []contracts.DownloadRequest

	for _, fileReq := range req.Files {
		fileInfo, err := s.GetFileInfo(ctx, fileReq.FilePath)
		if err != nil {
			logger.Warn("Failed to get file info", "path", fileReq.FilePath, "error", err)
			continue
		}

		// 使用统一的方法构建下载请求
		downloadReq := s.buildDownloadRequest(*fileInfo, fileReq.TargetDir, fileReq.AutoClassify, fileReq.Options)

		// 应用全局设置
		if req.TargetDir != "" && downloadReq.Directory == fileReq.TargetDir {
			downloadReq.Directory = req.TargetDir
		}
		if req.AutoClassify {
			downloadReq.AutoClassify = true
		}

		downloadRequests = append(downloadRequests, downloadReq)
	}

	batchReq := contracts.BatchDownloadRequest{
		Items:        downloadRequests,
		Directory:    req.TargetDir,
		VideoOnly:    req.VideoOnly,
		AutoClassify: req.AutoClassify,
	}

	return s.downloadService.CreateBatchDownload(ctx, batchReq)
}

// DownloadDirectory 下载目录
func (s *AppFileService) DownloadDirectory(ctx context.Context, req contracts.DirectoryDownloadRequest) (*contracts.BatchDownloadResponse, error) {
	// 检查下载服务是否可用
	if s.downloadService == nil {
		return nil, fmt.Errorf("download service not available")
	}

	// 获取目录下的所有文件
	listReq := contracts.FileListRequest{
		Path:      req.DirectoryPath,
		Recursive: req.Recursive,
		VideoOnly: req.VideoOnly,
		PageSize:  10000,
	}

	listResp, err := s.ListFiles(ctx, listReq)
	if err != nil {
		return nil, fmt.Errorf("failed to list directory: %w", err)
	}

	// 转换为下载请求
	var downloadRequests []contracts.DownloadRequest
	for _, file := range listResp.Files {
		// 动态获取真实的下载URL（ListFiles返回的文件InternalURL为空，采用延迟加载）
		logger.Debug("Getting download URL for file in directory", "file", file.Name, "path", file.Path, "size", file.Size)
		internalURL, _ := s.getRealDownloadURLs(file.Path)

		// 填充InternalURL以便使用统一的构建方法
		file.InternalURL = internalURL

		// 使用统一的方法构建下载请求
		downloadReq := s.buildDownloadRequest(file, req.TargetDir, req.AutoClassify, nil)

		downloadRequests = append(downloadRequests, downloadReq)
		logger.Debug("Download request created", "file", file.Name, "fileSize", downloadReq.FileSize)
	}

	batchReq := contracts.BatchDownloadRequest{
		Items:        downloadRequests,
		Directory:    req.TargetDir,
		VideoOnly:    req.VideoOnly,
		AutoClassify: req.AutoClassify,
	}

	return s.downloadService.CreateBatchDownload(ctx, batchReq)
}
