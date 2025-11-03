package rename

import (
	"encoding/json"
	"testing"
)

// TestSuggestionJSONCompatibility_TVShow 测试TV剧集的JSON序列化
func TestSuggestionJSONCompatibility_TVShow(t *testing.T) {
	season := 1
	episode := 5
	suggestion := &Suggestion{
		OriginalPath: "/path/to/file.mkv",
		NewName:      "Breaking Bad - S01E05.mkv",
		NewPath:      "/path/to/Breaking Bad - S01E05.mkv",
		MediaType:    MediaTypeTV,
		Title:        "Breaking Bad",
		TitleCN:      "绝命毒师",
		Year:         2008,
		Season:       &season,
		Episode:      &episode,
		EpisodeTitle: "Gray Matter",
		TMDBID:       1396,
		Confidence:   0.95,
		Source:       SourceTMDB,
		RawResponse:  "debug info",
	}

	data, err := json.Marshal(suggestion)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// 解析为map检查字段
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// 验证关键字段
	if result["new_name"] != "Breaking Bad - S01E05.mkv" {
		t.Errorf("Expected new_name, got: %v", result["new_name"])
	}

	if result["media_type"] != "tv" {
		t.Errorf("Expected media_type=tv, got: %v", result["media_type"])
	}

	// 验证 Season 和 Episode 是 int 类型而非 null
	seasonValue, ok := result["season"].(float64) // JSON numbers are float64
	if !ok || int(seasonValue) != 1 {
		t.Errorf("Expected season=1 (int), got: %v (type: %T)", result["season"], result["season"])
	}

	episodeValue, ok := result["episode"].(float64)
	if !ok || int(episodeValue) != 5 {
		t.Errorf("Expected episode=5 (int), got: %v (type: %T)", result["episode"], result["episode"])
	}

	// 验证新增的LLM字段存在
	if result["title_cn"] != "绝命毒师" {
		t.Errorf("Expected title_cn, got: %v", result["title_cn"])
	}

	if result["episode_title"] != "Gray Matter" {
		t.Errorf("Expected episode_title, got: %v", result["episode_title"])
	}

	if result["source"] != "tmdb" {
		t.Errorf("Expected source, got: %v", result["source"])
	}

	// 验证 RawResponse 不出现在JSON中
	if _, exists := result["raw_response"]; exists {
		t.Error("RawResponse should not be serialized (json:\"-\")")
	}

	// 打印完整JSON用于验证
	t.Logf("TV Show JSON: %s", string(data))
}

// TestSuggestionJSONCompatibility_Movie 测试电影的JSON序列化
func TestSuggestionJSONCompatibility_Movie(t *testing.T) {
	suggestion := &Suggestion{
		NewName:    "The Matrix (1999).mkv",
		NewPath:    "/movies/The Matrix (1999).mkv",
		MediaType:  MediaTypeMovie,
		Title:      "The Matrix",
		Year:       1999,
		Season:     nil, // 电影无季度
		Episode:    nil, // 电影无集数
		TMDBID:     603,
		Confidence: 0.98,
		Source:     SourceTMDB,
	}

	data, err := json.Marshal(suggestion)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// 验证电影类型
	if result["media_type"] != "movie" {
		t.Errorf("Expected media_type=movie, got: %v", result["media_type"])
	}

	// 验证 Season 和 Episode 在电影中的表现
	// 由于使用了 omitempty，nil值转为0后可能不出现，或者出现为0
	if season, exists := result["season"]; exists {
		seasonValue, ok := season.(float64)
		if !ok || int(seasonValue) != 0 {
			t.Errorf("Expected season=0 or omitted for movie, got: %v (type: %T)", season, season)
		}
	}

	if episode, exists := result["episode"]; exists {
		episodeValue, ok := episode.(float64)
		if !ok || int(episodeValue) != 0 {
			t.Errorf("Expected episode=0 or omitted for movie, got: %v (type: %T)", episode, episode)
		}
	}

	// 打印完整JSON用于验证
	t.Logf("Movie JSON: %s", string(data))
}

// TestSuggestionJSONCompatibility_LLMSource 测试LLM来源的序列化
func TestSuggestionJSONCompatibility_LLMSource(t *testing.T) {
	season := 2
	episode := 10
	suggestion := &Suggestion{
		NewName:      "Game of Thrones - S02E10.mkv",
		NewPath:      "/tv/Game of Thrones/Season 02/Game of Thrones - S02E10.mkv",
		MediaType:    MediaTypeTV,
		Title:        "Game of Thrones",
		TitleCN:      "权力的游戏",
		Year:         2011,
		Season:       &season,
		Episode:      &episode,
		EpisodeTitle: "Valar Morghulis",
		TMDBID:       1399,
		Confidence:   0.92,
		Source:       SourceLLM, // LLM来源
	}

	data, err := json.Marshal(suggestion)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// 验证LLM特有字段
	if result["source"] != "llm" {
		t.Errorf("Expected source=llm, got: %v", result["source"])
	}

	if result["title_cn"] != "权力的游戏" {
		t.Errorf("Expected title_cn, got: %v", result["title_cn"])
	}

	if result["episode_title"] != "Valar Morghulis" {
		t.Errorf("Expected episode_title, got: %v", result["episode_title"])
	}

	t.Logf("LLM Source JSON: %s", string(data))
}

// TestSuggestionUnmarshalJSON 测试反序列化（从API响应解析）
func TestSuggestionUnmarshalJSON(t *testing.T) {
	jsonData := `{
		"new_name": "The Office - S03E05.mkv",
		"new_path": "/tv/The Office - S03E05.mkv",
		"media_type": "tv",
		"title": "The Office",
		"year": 2005,
		"season": 3,
		"episode": 5,
		"tmdb_id": 2316,
		"confidence": 0.89,
		"source": "tmdb"
	}`

	var suggestion Suggestion
	if err := json.Unmarshal([]byte(jsonData), &suggestion); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// 验证基础字段
	if suggestion.NewName != "The Office - S03E05.mkv" {
		t.Errorf("Expected NewName, got: %s", suggestion.NewName)
	}

	if suggestion.MediaType != MediaTypeTV {
		t.Errorf("Expected MediaType=tv, got: %v", suggestion.MediaType)
	}

	// 验证 Season 和 Episode 被正确解析为指针
	if suggestion.Season == nil || *suggestion.Season != 3 {
		t.Errorf("Expected Season=3, got: %v", suggestion.Season)
	}

	if suggestion.Episode == nil || *suggestion.Episode != 5 {
		t.Errorf("Expected Episode=5, got: %v", suggestion.Episode)
	}

	if suggestion.Source != SourceTMDB {
		t.Errorf("Expected Source=tmdb, got: %v", suggestion.Source)
	}
}

// BenchmarkSuggestionMarshalJSON 性能基准测试
func BenchmarkSuggestionMarshalJSON(b *testing.B) {
	season := 1
	episode := 1
	suggestion := &Suggestion{
		NewName:    "Test Show - S01E01.mkv",
		NewPath:    "/tv/Test Show - S01E01.mkv",
		MediaType:  MediaTypeTV,
		Title:      "Test Show",
		Year:       2020,
		Season:     &season,
		Episode:    &episode,
		TMDBID:     12345,
		Confidence: 0.95,
		Source:     SourceTMDB,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(suggestion)
	}
}
