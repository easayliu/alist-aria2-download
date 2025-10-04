package executor

import (
	"fmt"
	"sync"

	"github.com/easayliu/alist-aria2-download/internal/application/services"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/aria2"
)

// BatchDownloadExecutor 批量下载执行器 - 统一处理批量下载逻辑
type BatchDownloadExecutor struct {
	aria2Client *aria2.Client
	concurrency int // 并发数
}

// NewBatchDownloadExecutor 创建批量下载执行器
func NewBatchDownloadExecutor(aria2Client *aria2.Client, concurrency int) *BatchDownloadExecutor {
	if concurrency <= 0 {
		concurrency = 5 // 默认5个并发
	}
	return &BatchDownloadExecutor{
		aria2Client: aria2Client,
		concurrency: concurrency,
	}
}

// DownloadResult 单个下载结果
type DownloadResult struct {
	FileName     string
	Path         string
	MediaType    string
	DownloadPath string
	Status       string // "success" or "failed"
	GID          string
	Error        string
}

// BatchDownloadResult 批量下载结果
type BatchDownloadResult struct {
	TotalCount   int
	SuccessCount int
	FailCount    int
	Results      []DownloadResult
}

// Execute 执行批量下载
func (e *BatchDownloadExecutor) Execute(files []services.FileInfo) *BatchDownloadResult {
	result := &BatchDownloadResult{
		TotalCount: len(files),
		Results:    make([]DownloadResult, 0, len(files)),
	}

	if len(files) == 0 {
		return result
	}

	// 使用channel控制并发
	semaphore := make(chan struct{}, e.concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, file := range files {
		wg.Add(1)
		go func(f services.FileInfo) {
			defer wg.Done()

			// 获取信号量
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// 执行下载
			downloadResult := e.downloadSingleFile(f)

			// 线程安全地添加结果
			mu.Lock()
			result.Results = append(result.Results, downloadResult)
			if downloadResult.Status == "success" {
				result.SuccessCount++
			} else {
				result.FailCount++
			}
			mu.Unlock()
		}(file)
	}

	wg.Wait()
	return result
}

// downloadSingleFile 下载单个文件
func (e *BatchDownloadExecutor) downloadSingleFile(file services.FileInfo) DownloadResult {
	options := map[string]interface{}{
		"dir": file.DownloadPath,
		"out": file.Name,
	}

	gid, err := e.aria2Client.AddURI(file.InternalURL, options)
	if err != nil {
		return DownloadResult{
			FileName:     file.Name,
			Path:         file.Path,
			MediaType:    string(file.MediaType),
			DownloadPath: file.DownloadPath,
			Status:       "failed",
			Error:        err.Error(),
		}
	}

	return DownloadResult{
		FileName:     file.Name,
		Path:         file.Path,
		MediaType:    string(file.MediaType),
		DownloadPath: file.DownloadPath,
		Status:       "success",
		GID:          gid,
	}
}

// ExecuteSequential 串行执行批量下载(用于需要严格顺序的场景)
func (e *BatchDownloadExecutor) ExecuteSequential(files []services.FileInfo) *BatchDownloadResult {
	result := &BatchDownloadResult{
		TotalCount: len(files),
		Results:    make([]DownloadResult, 0, len(files)),
	}

	for _, file := range files {
		downloadResult := e.downloadSingleFile(file)
		result.Results = append(result.Results, downloadResult)

		if downloadResult.Status == "success" {
			result.SuccessCount++
		} else {
			result.FailCount++
		}
	}

	return result
}

// FormatResultSummary 格式化结果摘要
func (r *BatchDownloadResult) FormatResultSummary() string {
	return fmt.Sprintf("Total: %d, Success: %d, Failed: %d",
		r.TotalCount, r.SuccessCount, r.FailCount)
}
