package services

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

		downloadReq := contracts.DownloadRequest{
			URL:          fileInfo.InternalURL,
			Filename:     fileInfo.Name,
			Directory:    fileReq.TargetDir,
			Options:      fileReq.Options,
			AutoClassify: fileReq.AutoClassify,
		}

		// 应用全局设置
		if req.TargetDir != "" && downloadReq.Directory == "" {
			downloadReq.Directory = req.TargetDir
		}
		if req.AutoClassify {
			downloadReq.AutoClassify = true
		}

		if downloadReq.Directory == "" {
			downloadReq.Directory = s.GenerateDownloadPath(*fileInfo)
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
		// 动态获取真实的下载URL
		logger.Debug("Getting download URL for file in directory", "file", file.Name, "path", file.Path)
		internalURL, _ := s.getRealDownloadURLs(file.Path)
		
		downloadReq := contracts.DownloadRequest{
			URL:          internalURL,
			Filename:     file.Name,
			Directory:    req.TargetDir,
			AutoClassify: req.AutoClassify,
		}

		if downloadReq.Directory == "" {
			downloadReq.Directory = s.GenerateDownloadPath(file)
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