package filename

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/easayliu/alist-aria2-download/pkg/logger"
)

// HybridSuggester 混合推断器（TMDB + LLM）
// 结合TMDB数据库和LLM能力，提供更准确的文件名推断
type HybridSuggester struct {
	tmdbSuggester TMDBSuggester  // TMDB推断器接口
	llmSuggester  *LLMSuggester  // LLM推断器
	strategy      HybridStrategy // 混合策略
}

// HybridStrategy 混合策略
type HybridStrategy int

const (
	// TMDBFirst TMDB优先，失败时使用LLM
	TMDBFirst HybridStrategy = iota
	// LLMFirst LLM优先，失败时使用TMDB
	LLMFirst
	// TMDBOnly 仅使用TMDB
	TMDBOnly
	// LLMOnly 仅使用LLM
	LLMOnly
	// Compare 同时使用并比较结果
	Compare
)

// NewHybridSuggester 创建混合推断器
func NewHybridSuggester(tmdbSuggester TMDBSuggester, llmSuggester *LLMSuggester, strategy HybridStrategy) *HybridSuggester {
	return &HybridSuggester{
		tmdbSuggester: tmdbSuggester,
		llmSuggester:  llmSuggester,
		strategy:      strategy,
	}
}

// SuggestFileName 推断文件名
func (s *HybridSuggester) SuggestFileName(ctx context.Context, req FileNameRequest) (*FileNameSuggestion, error) {
	logger.Info("Hybrid inference started",
		"strategy", s.strategy,
		"originalName", req.OriginalName)

	switch s.strategy {
	case TMDBFirst:
		return s.suggestWithTMDBFirst(ctx, req)
	case LLMFirst:
		return s.suggestWithLLMFirst(ctx, req)
	case TMDBOnly:
		return s.suggestWithTMDBOnly(ctx, req)
	case LLMOnly:
		return s.suggestWithLLMOnly(ctx, req)
	default:
		return nil, fmt.Errorf("unsupported hybrid strategy: %d", s.strategy)
	}
}

// SuggestFileNameWithCompare 比较策略（返回多个结果供用户选择）
func (s *HybridSuggester) SuggestFileNameWithCompare(ctx context.Context, req FileNameRequest) ([]*FileNameSuggestion, error) {
	logger.Info("Compare mode inference started", "originalName", req.OriginalName)

	var results []*FileNameSuggestion

	// 尝试TMDB
	tmdbResult, tmdbErr := s.tryTMDB(ctx, req)
	if tmdbErr == nil && tmdbResult != nil {
		results = append(results, tmdbResult)
		logger.Info("TMDB inference succeeded", "confidence", tmdbResult.Confidence)
	} else {
		logger.Warn("TMDB inference failed", "error", tmdbErr)
	}

	// 尝试LLM
	llmResult, llmErr := s.tryLLM(ctx, req)
	if llmErr == nil && llmResult != nil {
		results = append(results, llmResult)
		logger.Info("LLM inference succeeded", "confidence", llmResult.Confidence)
	} else {
		logger.Warn("LLM inference failed", "error", llmErr)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("both TMDB and LLM failed: tmdb_err=%v, llm_err=%v", tmdbErr, llmErr)
	}

	return results, nil
}

// suggestWithTMDBFirst TMDB优先策略
func (s *HybridSuggester) suggestWithTMDBFirst(ctx context.Context, req FileNameRequest) (*FileNameSuggestion, error) {
	logger.Info("Using TMDB-first strategy")

	// 1. 尝试TMDB
	tmdbResult, tmdbErr := s.tryTMDB(ctx, req)
	if tmdbErr == nil && tmdbResult != nil && tmdbResult.Confidence > 0.7 {
		// TMDB成功且置信度高，直接返回
		logger.Info("TMDB inference succeeded with high confidence, using it", "confidence", tmdbResult.Confidence)
		return tmdbResult, nil
	}

	// 2. TMDB失败或置信度低，fallback到LLM
	logger.Info("TMDB failed or low confidence, switching to LLM",
		"tmdbError", tmdbErr,
		"tmdbConfidence", func() float32 {
			if tmdbResult != nil {
				return tmdbResult.Confidence
			}
			return 0
		}())

	llmResult, llmErr := s.tryLLM(ctx, req)
	if llmErr != nil {
		// 如果LLM也失败，但TMDB有结果（虽然置信度低），返回TMDB结果
		if tmdbResult != nil {
			logger.Warn("LLM inference failed, using low-confidence TMDB result", "error", llmErr)
			return tmdbResult, nil
		}
		return nil, fmt.Errorf("both TMDB and LLM failed: tmdb_err=%v, llm_err=%v", tmdbErr, llmErr)
	}

	return llmResult, nil
}

// suggestWithLLMFirst LLM优先策略
func (s *HybridSuggester) suggestWithLLMFirst(ctx context.Context, req FileNameRequest) (*FileNameSuggestion, error) {
	logger.Info("Using LLM-first strategy")

	// 1. 尝试LLM
	llmResult, llmErr := s.tryLLM(ctx, req)
	if llmErr == nil && llmResult != nil && llmResult.Confidence > 0.7 {
		// LLM成功且置信度高，直接返回
		logger.Info("LLM inference succeeded with high confidence, using it", "confidence", llmResult.Confidence)
		return llmResult, nil
	}

	// 2. LLM失败或置信度低，fallback到TMDB
	logger.Info("LLM failed or low confidence, switching to TMDB",
		"llmError", llmErr,
		"llmConfidence", func() float32 {
			if llmResult != nil {
				return llmResult.Confidence
			}
			return 0
		}())

	tmdbResult, tmdbErr := s.tryTMDB(ctx, req)
	if tmdbErr != nil {
		// 如果TMDB也失败，但LLM有结果（虽然置信度低），返回LLM结果
		if llmResult != nil {
			logger.Warn("TMDB inference failed, using low-confidence LLM result", "error", tmdbErr)
			return llmResult, nil
		}
		return nil, fmt.Errorf("both LLM and TMDB failed: llm_err=%v, tmdb_err=%v", llmErr, tmdbErr)
	}

	return tmdbResult, nil
}

// suggestWithTMDBOnly 仅使用TMDB
func (s *HybridSuggester) suggestWithTMDBOnly(ctx context.Context, req FileNameRequest) (*FileNameSuggestion, error) {
	logger.Info("Using TMDB-only mode")
	return s.tryTMDB(ctx, req)
}

// suggestWithLLMOnly 仅使用LLM
func (s *HybridSuggester) suggestWithLLMOnly(ctx context.Context, req FileNameRequest) (*FileNameSuggestion, error) {
	logger.Info("Using LLM-only mode")
	return s.tryLLM(ctx, req)
}

// tryTMDB 尝试使用TMDB推断
func (s *HybridSuggester) tryTMDB(ctx context.Context, req FileNameRequest) (*FileNameSuggestion, error) {
	// 使用TMDB的SearchAndSuggest方法
	fullPath := req.OriginalName
	if req.FilePath != "" {
		fullPath = req.FilePath
	}

	suggestions, err := s.tmdbSuggester.SearchAndSuggest(ctx, fullPath)
	if err != nil {
		return nil, fmt.Errorf("TMDB推断失败: %w", err)
	}

	if len(suggestions) == 0 {
		return nil, fmt.Errorf("TMDB未找到匹配结果")
	}

	// 使用第一个结果（置信度最高）
	tmdbSuggestion := suggestions[0]

	// 转换为统一的FileNameSuggestion格式
	return convertTMDBToFileNameSuggestion(&tmdbSuggestion), nil
}

// tryLLM 尝试使用LLM推断
func (s *HybridSuggester) tryLLM(ctx context.Context, req FileNameRequest) (*FileNameSuggestion, error) {
	return s.llmSuggester.SuggestFileName(ctx, req)
}

// convertTMDBToFileNameSuggestion 将TMDB结果转换为统一的FileNameSuggestion格式
func convertTMDBToFileNameSuggestion(tmdbSuggestion *TMDBSuggestedName) *FileNameSuggestion {
	result := &FileNameSuggestion{
		Title:      tmdbSuggestion.Title,
		Year:       tmdbSuggestion.Year,
		Confidence: float32(tmdbSuggestion.Confidence),
		Source:     "tmdb",
	}

	// 设置媒体类型
	if string(tmdbSuggestion.MediaType) == "tv" {
		result.MediaType = "tv"
		// 设置季集数
		if tmdbSuggestion.Season > 0 {
			season := tmdbSuggestion.Season
			result.Season = &season
		}
		if tmdbSuggestion.Episode > 0 {
			episode := tmdbSuggestion.Episode
			result.Episode = &episode
		}
	} else {
		result.MediaType = "movie"
	}

	return result
}

// ToTMDBSuggestedName 将FileNameSuggestion转换回TMDB的TMDBSuggestedName格式
// 用于与现有系统集成
func (s *FileNameSuggestion) ToTMDBSuggestedName(originalPath string) *TMDBSuggestedName {
	result := &TMDBSuggestedName{
		Title:      s.Title,
		Year:       s.Year,
		Confidence: float64(s.Confidence),
	}

	// 设置媒体类型
	if s.MediaType == "tv" {
		if s.Season != nil {
			result.Season = *s.Season
		}
		if s.Episode != nil {
			result.Episode = *s.Episode
		}
	}

	// 构建新文件名
	extension := filepath.Ext(originalPath)
	result.NewName = s.ToEmbyFormat(extension)

	// 构建新路径（简单处理，保持在原目录）
	dir := filepath.Dir(originalPath)
	result.NewPath = filepath.Join(dir, result.NewName)

	return result
}

// GetStrategyName 获取策略名称（用于日志和调试）
func (strategy HybridStrategy) String() string {
	switch strategy {
	case TMDBFirst:
		return "TMDB优先"
	case LLMFirst:
		return "LLM优先"
	case TMDBOnly:
		return "仅TMDB"
	case LLMOnly:
		return "仅LLM"
	case Compare:
		return "比较模式"
	default:
		return "未知策略"
	}
}
