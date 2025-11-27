package rename

import (
	"encoding/json"

	"github.com/easayliu/alist-aria2-download/internal/infrastructure/tmdb"
)

// Suggestion 统一的重命名建议结构（领域模型）
// 用于TMDB、LLM等所有重命名场景
type Suggestion struct {
	// ========== 基础信息 ==========
	OriginalPath string `json:"original_path,omitempty"` // 原始文件路径
	NewName      string `json:"new_name"`                // 新文件名（不含路径）
	NewPath      string `json:"new_path"`                // 新完整路径

	// ========== 媒体信息 ==========
	MediaType MediaType `json:"media_type"`         // "movie" | "tv"
	Title     string    `json:"title"`              // 英文标题
	TitleCN   string    `json:"title_cn,omitempty"` // 中文标题（可选，LLM专用）
	Year      int       `json:"year"`               // 年份

	// ========== 剧集信息（TV专用）==========
	Season       *int   `json:"-"`                       // 季度（指针表示可选）- 通过MarshalJSON自定义序列化
	Episode      *int   `json:"-"`                       // 集数（指针表示可选）- 通过MarshalJSON自定义序列化
	EpisodeTitle string `json:"episode_title,omitempty"` // 集数标题（可选，LLM专用）

	// ========== 元数据 ==========
	TMDBID     int     `json:"tmdb_id"`          // TMDB ID（TMDB专用，0表示无）
	Confidence float64 `json:"confidence"`       // 置信度 0.0-1.0
	Source     Source  `json:"source,omitempty"` // 数据来源：TMDB/LLM/Hybrid

	// ========== 调试信息（不序列化到API）==========
	RawResponse string `json:"-"` // LLM原始响应（调试用）

	// ========== 跳过标记 ==========
	Skipped    bool   `json:"skipped,omitempty"`     // 是否跳过（已符合标准格式）
	SkipReason string `json:"skip_reason,omitempty"` // 跳过原因
}

// MediaType 媒体类型
type MediaType string

const (
	MediaTypeMovie MediaType = "movie"
	MediaTypeTV    MediaType = "tv"
)

// Source 数据来源
type Source string

const (
	SourceTMDB   Source = "tmdb"
	SourceLLM    Source = "llm"
	SourceHybrid Source = "hybrid"
)

// ToTMDBMediaType 转换为TMDB的MediaType（用于API调用）
func (m MediaType) ToTMDBMediaType() tmdb.MediaType {
	return tmdb.MediaType(m)
}

// FromTMDBMediaType 从TMDB的MediaType转换
func FromTMDBMediaType(t tmdb.MediaType) MediaType {
	return MediaType(t)
}

// GetSeasonNumber 获取季度数字（如果为nil返回0）
func (s *Suggestion) GetSeasonNumber() int {
	if s.Season != nil {
		return *s.Season
	}
	return 0
}

// GetEpisodeNumber 获取集数数字（如果为nil返回0）
func (s *Suggestion) GetEpisodeNumber() int {
	if s.Episode != nil {
		return *s.Episode
	}
	return 0
}

// SetSeason 设置季度（辅助方法）
func (s *Suggestion) SetSeason(season int) {
	s.Season = &season
}

// SetEpisode 设置集数（辅助方法）
func (s *Suggestion) SetEpisode(episode int) {
	s.Episode = &episode
}

// MarshalJSON 自定义JSON序列化，保持API向后兼容
// Season和Episode输出为int类型（0而非null），兼容现有客户端
func (s *Suggestion) MarshalJSON() ([]byte, error) {
	// 使用type alias避免无限递归
	type Alias Suggestion

	// 创建临时结构体，添加Season和Episode为int类型
	return json.Marshal(&struct {
		Season  int `json:"season,omitempty"`
		Episode int `json:"episode,omitempty"`
		*Alias
	}{
		Season:  s.GetSeasonNumber(),  // nil -> 0
		Episode: s.GetEpisodeNumber(), // nil -> 0
		Alias:   (*Alias)(s),
	})
}

// UnmarshalJSON 自定义JSON反序列化
// 将int类型的season/episode转换为指针类型
func (s *Suggestion) UnmarshalJSON(data []byte) error {
	// 使用type alias避免无限递归
	type Alias Suggestion

	// 临时结构体接收int类型的season/episode
	aux := &struct {
		Season  int `json:"season,omitempty"`
		Episode int `json:"episode,omitempty"`
		*Alias
	}{
		Alias: (*Alias)(s),
	}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	// 转换int为指针（如果非0）
	if aux.Season > 0 {
		s.Season = &aux.Season
	}
	if aux.Episode > 0 {
		s.Episode = &aux.Episode
	}

	return nil
}
