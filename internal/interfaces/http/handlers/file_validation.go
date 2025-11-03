package handlers

// GetYesterdayFilesRequest 获取昨天文件请求参数
type GetYesterdayFilesRequest struct {
	Path string `form:"path" json:"path"`
}

// DownloadPathRequest 下载路径请求参数
type DownloadPathRequest struct {
	Path      string `json:"path" binding:"required"`
	Recursive bool   `json:"recursive"`
	Preview   bool   `json:"preview"` // 预览模式，只生成路径不下载
}

// DownloadYesterdayFilesRequest 下载昨天文件请求参数
type DownloadYesterdayFilesRequest struct {
	Path    string `form:"path" json:"path"`
	Preview bool   `form:"preview" json:"preview"` // 预览模式
}

// FileListRequest 列出文件请求参数
type FileListRequest struct {
	Path      string `json:"path"` // 路径，为空时使用默认路径
	Page      int    `json:"page"`
	PerPage   int    `json:"per_page"`
	VideoOnly bool   `json:"video_only"` // 是否只显示视频文件
}

// ManualDownloadRequest 手动下载请求参数
type ManualDownloadRequest struct {
	Path      string `json:"path" example:"/downloads"`                           // 搜索路径（可选，为空时使用配置的默认路径）
	HoursAgo  int    `json:"hours_ago" example:"24"`                              // 最近多少小时内的文件（可选，默认24小时）
	VideoOnly bool   `json:"video_only" example:"false"`                          // 是否只下载视频文件
	Preview   bool   `json:"preview" example:"false"`                             // 是否预览模式
	StartTime string `json:"start_time,omitempty" example:"2023-12-01T00:00:00Z"` // 开始时间（可选，ISO 8601格式）
	EndTime   string `json:"end_time,omitempty" example:"2023-12-02T00:00:00Z"`   // 结束时间（可选，ISO 8601格式）
}
