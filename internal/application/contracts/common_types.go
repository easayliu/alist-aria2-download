package contracts

import "time"

// TimeRange 时间范围
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// SortOrder 排序方向
type SortOrder string

const (
	SortOrderAsc  SortOrder = "asc"
	SortOrderDesc SortOrder = "desc"
)

// SortField 排序字段
type SortField string

const (
	SortFieldName     SortField = "name"
	SortFieldSize     SortField = "size"
	SortFieldModified SortField = "modified"
	SortFieldCreated  SortField = "created"
	SortFieldStatus   SortField = "status"
	SortFieldProgress SortField = "progress"
)

// ErrorCode 业务错误码
type ErrorCode string

const (
	ErrorCodeInvalidRequest   ErrorCode = "INVALID_REQUEST"
	ErrorCodeNotFound         ErrorCode = "NOT_FOUND"
	ErrorCodeUnauthorized     ErrorCode = "UNAUTHORIZED"
	ErrorCodeForbidden        ErrorCode = "FORBIDDEN"
	ErrorCodeConflict         ErrorCode = "CONFLICT"
	ErrorCodeInternalError    ErrorCode = "INTERNAL_ERROR"
	ErrorCodeServiceUnavailable ErrorCode = "SERVICE_UNAVAILABLE"
	ErrorCodeTimeout          ErrorCode = "TIMEOUT"
	ErrorCodeRateLimit        ErrorCode = "RATE_LIMIT"
	ErrorCodeQuotaExceeded    ErrorCode = "QUOTA_EXCEEDED"
)

// ServiceError 业务错误
type ServiceError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
	Cause   error     `json:"-"`
}

func (e *ServiceError) Error() string {
	return string(e.Code) + ": " + e.Message
}

// NewServiceError 创建业务错误
func NewServiceError(code ErrorCode, message string) *ServiceError {
	return &ServiceError{
		Code:    code,
		Message: message,
	}
}

// NewServiceErrorWithCause 创建带原因的业务错误
func NewServiceErrorWithCause(code ErrorCode, message string, cause error) *ServiceError {
	return &ServiceError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

// NewServiceErrorWithDetails 创建带详情的业务错误
func NewServiceErrorWithDetails(code ErrorCode, message string, details map[string]interface{}) *ServiceError {
	return &ServiceError{
		Code:    code,
		Message: message,
		Details: details,
	}
}

// HealthStatus 健康状态
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnknown   HealthStatus = "unknown"
)

// ComponentHealth 组件健康状态
type ComponentHealth struct {
	Name      string                 `json:"name"`
	Status    HealthStatus           `json:"status"`
	Message   string                 `json:"message,omitempty"`
	LastCheck time.Time              `json:"last_check"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// SystemHealth 系统健康状态
type SystemHealth struct {
	Status     HealthStatus       `json:"status"`
	Components []ComponentHealth  `json:"components"`
	Timestamp  time.Time          `json:"timestamp"`
	Uptime     time.Duration      `json:"uptime"`
	Version    string             `json:"version"`
}

// Metrics 系统指标
type Metrics struct {
	// 通用指标
	RequestCount   int64     `json:"request_count"`
	ErrorCount     int64     `json:"error_count"`
	ResponseTimeMs int64     `json:"response_time_ms"`
	Timestamp      time.Time `json:"timestamp"`
	
	// 下载指标
	ActiveDownloads    int   `json:"active_downloads,omitempty"`
	CompletedDownloads int64 `json:"completed_downloads,omitempty"`
	FailedDownloads    int64 `json:"failed_downloads,omitempty"`
	TotalDownloadSize  int64 `json:"total_download_size,omitempty"`
	AvgDownloadSpeed   int64 `json:"avg_download_speed,omitempty"`
	
	// 任务指标
	ActiveTasks    int   `json:"active_tasks,omitempty"`
	CompletedTasks int64 `json:"completed_tasks,omitempty"`
	FailedTasks    int64 `json:"failed_tasks,omitempty"`
	
	// 系统指标
	CPUUsage    float64 `json:"cpu_usage,omitempty"`
	MemoryUsage int64   `json:"memory_usage,omitempty"`
	DiskUsage   int64   `json:"disk_usage,omitempty"`
	
	// 自定义指标
	Custom map[string]interface{} `json:"custom,omitempty"`
}

// PaginationRequest 分页请求
type PaginationRequest struct {
	Page     int `json:"page" validate:"min=1"`
	PageSize int `json:"page_size" validate:"min=1,max=1000"`
}

// PaginationResponse 分页响应
type PaginationResponse struct {
	Page       int  `json:"page"`
	PageSize   int  `json:"page_size"`
	Total      int  `json:"total"`
	TotalPages int  `json:"total_pages"`
	HasNext    bool `json:"has_next"`
	HasPrev    bool `json:"has_prev"`
}

// NewPaginationResponse 创建分页响应
func NewPaginationResponse(page, pageSize, total int) PaginationResponse {
	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}
	
	return PaginationResponse{
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    page < totalPages,
		HasPrev:    page > 1,
	}
}

// FilterRequest 通用过滤请求
type FilterRequest struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"` // eq, ne, gt, gte, lt, lte, in, like
	Value    interface{} `json:"value"`
}

// SortRequest 排序请求
type SortRequest struct {
	Field SortField `json:"field"`
	Order SortOrder `json:"order"`
}

// SearchRequest 搜索请求
type SearchRequest struct {
	Query   string          `json:"query"`
	Filters []FilterRequest `json:"filters,omitempty"`
	Sort    []SortRequest   `json:"sort,omitempty"`
	PaginationRequest
}