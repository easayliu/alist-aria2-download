package filename

// Batch processing constants
const (
	// DefaultBatchSize 默认批次大小
	DefaultBatchSize = 8

	// TokensPerFile 每个文件估算的token数
	TokensPerFile = 250

	// BaseTokenOverhead 基础token开销（prompt模板等）
	BaseTokenOverhead = 1000

	// MinTokenLimit 最小token限制
	MinTokenLimit = 2000

	// MaxTokenLimit 最大token限制（考虑模型限制）
	MaxTokenLimit = 20000

	// DefaultSeason 默认季度（当无法推断时）
	DefaultSeason = 1
)

// Confidence thresholds
const (
	// HighConfidenceThreshold 高置信度阈值
	HighConfidenceThreshold = 0.9

	// MediumConfidenceThreshold 中等置信度阈值
	MediumConfidenceThreshold = 0.7
)
