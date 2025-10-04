package errors

// ErrorCode 业务错误码
type ErrorCode string

const (
	ErrorCodeInvalidRequest     ErrorCode = "INVALID_REQUEST"
	ErrorCodeNotFound           ErrorCode = "NOT_FOUND"
	ErrorCodeUnauthorized       ErrorCode = "UNAUTHORIZED"
	ErrorCodeForbidden          ErrorCode = "FORBIDDEN"
	ErrorCodeConflict           ErrorCode = "CONFLICT"
	ErrorCodeInternalError      ErrorCode = "INTERNAL_ERROR"
	ErrorCodeServiceUnavailable ErrorCode = "SERVICE_UNAVAILABLE"
	ErrorCodeTimeout            ErrorCode = "TIMEOUT"
	ErrorCodeRateLimit          ErrorCode = "RATE_LIMIT"
	ErrorCodeQuotaExceeded      ErrorCode = "QUOTA_EXCEEDED"
)

// ServiceError 业务错误
type ServiceError struct {
	Code    ErrorCode              `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
	Cause   error                  `json:"-"`
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
