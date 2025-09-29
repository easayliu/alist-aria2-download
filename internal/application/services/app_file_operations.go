package services

import (
	"context"
	"fmt"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
)

// DownloadFile 下载单个文件
func (s *AppFileService) DownloadFile(ctx context.Context, req contracts.FileDownloadRequest) (*contracts.DownloadResponse, error) {
	logger.Info("📁 开始下载单个文件", "filePath", req.FilePath)
	
	// 检查下载服务是否可用
	if s.downloadService == nil {
		return nil, fmt.Errorf("download service not available")
	}
	
	// 获取文件信息
	fileInfo, err := s.GetFileInfo(ctx, req.FilePath)
	if err != nil {
		logger.Error("❌ 获取文件信息失败", "filePath", req.FilePath, "error", err)
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	logger.Info("📋 文件信息获取成功", 
		"fileName", fileInfo.Name,
		"fileSize", fileInfo.Size,
		"downloadURL", fileInfo.InternalURL)

	// 构建下载请求
	downloadReq := contracts.DownloadRequest{
		URL:          fileInfo.InternalURL,
		Filename:     fileInfo.Name,
		Directory:    req.TargetDir,
		Options:      req.Options,
		AutoClassify: req.AutoClassify,
	}

	if downloadReq.Directory == "" {
		downloadReq.Directory = s.GenerateDownloadPath(*fileInfo)
	}

	logger.Info("🚀 准备创建下载任务", 
		"url", downloadReq.URL,
		"filename", downloadReq.Filename,
		"directory", downloadReq.Directory)

	return s.downloadService.CreateDownload(ctx, downloadReq)
}

// ClassifyFiles 文件分类
func (s *AppFileService) ClassifyFiles(ctx context.Context, req contracts.FileClassificationRequest) (*contracts.FileClassificationResponse, error) {
	classified := make(map[string][]contracts.FileResponse)
	summary := contracts.ClassificationSummary{
		Categories: make(map[string]int),
	}

	for _, file := range req.Files {
		category := s.GetFileCategory(file.Name)
		classified[category] = append(classified[category], file)
		summary.Categories[category]++

		// 特殊分类统计
		switch category {
		case "movie":
			summary.MovieCount++
		case "tv":
			summary.TVCount++
		default:
			summary.OtherCount++
		}
	}

	return &contracts.FileClassificationResponse{
		ClassifiedFiles: classified,
		Summary:         summary,
	}, nil
}

// GetFilesByCategory 根据分类获取文件
func (s *AppFileService) GetFilesByCategory(ctx context.Context, path string, category string) (*contracts.FileListResponse, error) {
	listReq := contracts.FileListRequest{
		Path:      path,
		Recursive: true,
		PageSize:  10000,
	}

	listResp, err := s.ListFiles(ctx, listReq)
	if err != nil {
		return nil, err
	}

	// 按分类过滤
	var filteredFiles []contracts.FileResponse
	for _, file := range listResp.Files {
		if s.GetFileCategory(file.Name) == category {
			filteredFiles = append(filteredFiles, file)
		}
	}

	listResp.Files = filteredFiles
	listResp.TotalCount = len(filteredFiles)
	listResp.Summary = s.calculateFileSummary(filteredFiles)

	return listResp, nil
}