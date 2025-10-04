package handlers

import "github.com/easayliu/alist-aria2-download/internal/application/services"

// convertYesterdayToFileInfo 转换YesterdayFileInfo到FileInfo
// 这是一个内部辅助函数,用于在handlers包内部进行类型转换
func convertYesterdayToFileInfo(files []services.YesterdayFileInfo) []services.FileInfo {
	result := make([]services.FileInfo, 0, len(files))
	for _, file := range files {
		result = append(result, services.FileInfo{
			Name:         file.Name,
			Path:         file.Path,
			Size:         file.Size,
			Modified:     file.Modified,
			MediaType:    file.MediaType,
			DownloadPath: file.DownloadPath,
			InternalURL:  file.InternalURL,
		})
	}
	return result
}
