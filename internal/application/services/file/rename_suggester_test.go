package file

import (
	"testing"
)

// TestExtractTVInfoFromPath_CombinedShowAndSeason 测试从"剧集名+季度"组合目录中提取信息
func TestExtractTVInfoFromPath_CombinedShowAndSeason(t *testing.T) {
	rs := &RenameSuggester{
		tmdbClient: nil, // 不需要真实的client
	}

	tests := []struct {
		name           string
		path           string
		expectedShow   string
		expectedSeason int
	}{
		{
			name:           "新闻女王 S2 格式",
			path:           "/data/来自：分享/tvs/新闻女王 S2/X.W.N.W.2.2025.S02E06.2160p.HQ.60fps.WEB-DL.H265.10bit.HDR10.AAC-GyWEB.mp4",
			expectedShow:   "新闻女王",
			expectedSeason: 2,
		},
		{
			name:           "英文剧集 S01 格式",
			path:           "/data/shows/Breaking Bad S01/episode.mkv",
			expectedShow:   "Breaking Bad",
			expectedSeason: 1,
		},
		{
			name:           "中文剧集 S03 格式",
			path:           "/media/电视剧/庆余年 S03/episode01.mp4",
			expectedShow:   "庆余年",
			expectedSeason: 3,
		},
		{
			name:           "带空格的季度格式",
			path:           "/data/shows/The Office S05/episode.mkv",
			expectedShow:   "The Office",
			expectedSeason: 5,
		},
		{
			name:           "分离的目录结构",
			path:           "/data/shows/Friends/Season 10/episode.mkv",
			expectedShow:   "Friends",
			expectedSeason: 10,
		},
		{
			name:           "优先使用tvs根目录后的中文剧名",
			path:           "/data/来自：分享/tvs/权力的游戏/Game.of.Thrones.S08.2019.UHD.Blu-ray.2160p.10bit.DoVi.2Audio.TrueHD(Atmos).7.1.x265-beAst/Game.of.Thrones.S08E06.The.Iron.Throne.by.Wall-E@beAst.mkv",
			expectedShow:   "权力的游戏",
			expectedSeason: 8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			showName, season := rs.extractTVInfoFromPath(tt.path)

			if showName != tt.expectedShow {
				t.Errorf("extractTVInfoFromPath() showName = %v, want %v", showName, tt.expectedShow)
			}

			if season != tt.expectedSeason {
				t.Errorf("extractTVInfoFromPath() season = %v, want %v", season, tt.expectedSeason)
			}
		})
	}
}

// TestExtractTVInfoFromPath_ChineseSeasonFormat 测试中文季度格式
func TestExtractTVInfoFromPath_ChineseSeasonFormat(t *testing.T) {
	rs := &RenameSuggester{
		tmdbClient: nil,
	}

	tests := []struct {
		name           string
		path           string
		expectedShow   string
		expectedSeason int
	}{
		{
			name:           "重影第一季",
			path:           "/data/shows/重影第一季/episode.mkv",
			expectedShow:   "重影",
			expectedSeason: 1,
		},
		{
			name:           "三体第二季",
			path:           "/media/三体第二季/episode01.mp4",
			expectedShow:   "三体",
			expectedSeason: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			showName, season := rs.extractTVInfoFromPath(tt.path)

			if showName != tt.expectedShow {
				t.Errorf("extractTVInfoFromPath() showName = %v, want %v", showName, tt.expectedShow)
			}

			if season != tt.expectedSeason {
				t.Errorf("extractTVInfoFromPath() season = %v, want %v", season, tt.expectedSeason)
			}
		})
	}
}

// TestParseFileName_TVEpisode 测试文件名解析不应该作为后备方案
func TestParseFileName_ShouldNotBeUsedAsFallback(t *testing.T) {
	rs := &RenameSuggester{
		tmdbClient: nil,
	}

	// 这个路径应该从目录名提取"新闻女王"，而不是从文件名提取"X W N W 2"
	path := "/data/来自：分享/tvs/新闻女王 S2/X.W.N.W.2.2025.S02E06.2160p.HQ.60fps.WEB-DL.H265.10bit.HDR10.AAC-GyWEB.mp4"

	showName, season := rs.extractTVInfoFromPath(path)

	// 应该从目录中提取到正确的剧集名
	if showName == "" {
		t.Errorf("extractTVInfoFromPath() failed to extract show name from directory")
	}

	if showName == "X W N W 2" || showName == "X.W.N.W.2" {
		t.Errorf("extractTVInfoFromPath() incorrectly extracted show name from filename: %v", showName)
	}

	if showName != "新闻女王" {
		t.Errorf("extractTVInfoFromPath() showName = %v, want '新闻女王'", showName)
	}

	if season != 2 {
		t.Errorf("extractTVInfoFromPath() season = %v, want 2", season)
	}
}

// TestCalculateEpisodeNumber 测试分集集数计算
func TestCalculateEpisodeNumber(t *testing.T) {
	rs := &RenameSuggester{
		tmdbClient: nil,
	}

	tests := []struct {
		name            string
		baseNum         int
		part            string
		partsPerEpisode int
		expected        int
	}{
		// 2集模式（上/下）测试
		{"第1期上-2集模式", 1, "上", 2, 1},
		{"第1期下-2集模式", 1, "下", 2, 2},
		{"第2期上-2集模式", 2, "上", 2, 3},
		{"第2期下-2集模式", 2, "下", 2, 4},
		{"第3期上-2集模式", 3, "上", 2, 5},
		{"第3期下-2集模式", 3, "下", 2, 6},

		// 3集模式（上/中/下）测试
		{"第1期上-3集模式", 1, "上", 3, 1},
		{"第1期中-3集模式", 1, "中", 3, 2},
		{"第1期下-3集模式", 1, "下", 3, 3},
		{"第2期上-3集模式", 2, "上", 3, 4},
		{"第2期中-3集模式", 2, "中", 3, 5},
		{"第2期下-3集模式", 2, "下", 3, 6},

		// 无分集标记
		{"第1期无标记", 1, "", 2, 1},
		{"第2期无标记", 2, "", 2, 2},
		{"第5期无标记", 5, "", 0, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rs.calculateEpisodeNumber(tt.baseNum, tt.part, tt.partsPerEpisode)
			if result != tt.expected {
				t.Errorf("calculateEpisodeNumber(%d, %q, %d) = %d, want %d",
					tt.baseNum, tt.part, tt.partsPerEpisode, result, tt.expected)
			}
		})
	}
}

// TestDetectPartsPerEpisode 测试分集模式检测
func TestDetectPartsPerEpisode(t *testing.T) {
	rs := &RenameSuggester{
		tmdbClient: nil,
	}

	tests := []struct {
		name     string
		infos    map[string]*MediaInfo
		expected int
	}{
		{
			name: "检测2集模式（上/下）",
			infos: map[string]*MediaInfo{
				"file1": {Part: "上"},
				"file2": {Part: "下"},
				"file3": {Part: "上"},
				"file4": {Part: "下"},
			},
			expected: 2,
		},
		{
			name: "检测3集模式（上/中/下）",
			infos: map[string]*MediaInfo{
				"file1": {Part: "上"},
				"file2": {Part: "中"},
				"file3": {Part: "下"},
			},
			expected: 3,
		},
		{
			name: "无分集标记",
			infos: map[string]*MediaInfo{
				"file1": {Part: ""},
				"file2": {Part: ""},
			},
			expected: 0,
		},
		{
			name: "混合情况下检测到中则为3集",
			infos: map[string]*MediaInfo{
				"file1": {Part: "上"},
				"file2": {Part: ""},
				"file3": {Part: "中"},
				"file4": {Part: "下"},
			},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rs.detectPartsPerEpisode(tt.infos)
			if result != tt.expected {
				t.Errorf("detectPartsPerEpisode() = %d, want %d", result, tt.expected)
			}
		})
	}
}

// TestRecalculateEpisodesWithPartMode 测试集数重新计算
func TestRecalculateEpisodesWithPartMode(t *testing.T) {
	rs := &RenameSuggester{
		tmdbClient: nil,
	}

	// 测试2集模式
	t.Run("2集模式重新计算", func(t *testing.T) {
		infos := map[string]*MediaInfo{
			"file1": {BaseEpisode: 1, Part: "上", Episode: 1},
			"file2": {BaseEpisode: 1, Part: "下", Episode: 1}, // 原来错误地计算为1
			"file3": {BaseEpisode: 2, Part: "上", Episode: 2}, // 原来错误地计算为2
			"file4": {BaseEpisode: 2, Part: "下", Episode: 2}, // 原来错误地计算为2
		}

		rs.recalculateEpisodesWithPartMode(infos, 2)

		expectations := map[string]int{
			"file1": 1,
			"file2": 2,
			"file3": 3,
			"file4": 4,
		}

		for path, expected := range expectations {
			if infos[path].Episode != expected {
				t.Errorf("file %s: Episode = %d, want %d", path, infos[path].Episode, expected)
			}
		}
	})

	// 测试3集模式
	t.Run("3集模式重新计算", func(t *testing.T) {
		infos := map[string]*MediaInfo{
			"file1": {BaseEpisode: 1, Part: "上", Episode: 1},
			"file2": {BaseEpisode: 1, Part: "中", Episode: 1},
			"file3": {BaseEpisode: 1, Part: "下", Episode: 1},
			"file4": {BaseEpisode: 2, Part: "上", Episode: 2},
		}

		rs.recalculateEpisodesWithPartMode(infos, 3)

		expectations := map[string]int{
			"file1": 1,
			"file2": 2,
			"file3": 3,
			"file4": 4,
		}

		for path, expected := range expectations {
			if infos[path].Episode != expected {
				t.Errorf("file %s: Episode = %d, want %d", path, infos[path].Episode, expected)
			}
		}
	})
}

// TestBuildEmbyPath 测试Emby标准路径生成
func TestBuildEmbyPath(t *testing.T) {
	rs := &RenameSuggester{
		tmdbClient: nil,
	}

	tests := []struct {
		name         string
		originalPath string
		seriesName   string
		year         int
		season       int
		fileName     string
		expectedPath string
	}{
		{
			name:         "标准tvs目录结构",
			originalPath: "/data/来自：分享/tvs/新闻女王 S2/X.W.N.W.2.2025.S02E06.mp4",
			seriesName:   "新闻女王",
			year:         2024,
			season:       2,
			fileName:     "新闻女王 - S02E06 - 第六集.mp4",
			expectedPath: "/data/来自：分享/tvs/新闻女王/Season 02/新闻女王 - S02E06 - 第六集.mp4",
		},
		{
			name:         "剧集目录结构",
			originalPath: "/media/剧集/庆余年/Season 03/episode.mkv",
			seriesName:   "庆余年",
			year:         2024,
			season:       3,
			fileName:     "庆余年 - S03E01 - 第一集.mkv",
			expectedPath: "/media/剧集/庆余年/Season 03/庆余年 - S03E01 - 第一集.mkv",
		},
		{
			name:         "电视剧目录结构",
			originalPath: "/data/电视剧/权力的游戏/S08/episode.mkv",
			seriesName:   "权力的游戏",
			year:         2019,
			season:       8,
			fileName:     "权力的游戏 - S08E06 - The Iron Throne.mkv",
			expectedPath: "/data/电视剧/权力的游戏/Season 08/权力的游戏 - S08E06 - The Iron Throne.mkv",
		},
		{
			name:         "无TV根目录时保留原目录",
			originalPath: "/random/path/show/episode.mkv",
			seriesName:   "Some Show",
			year:         2024,
			season:       1,
			fileName:     "Some Show - S01E01.mkv",
			expectedPath: "/random/path/show/Some Show - S01E01.mkv",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rs.buildEmbyPath(tt.originalPath, tt.seriesName, tt.year, tt.season, tt.fileName)

			if result != tt.expectedPath {
				t.Errorf("buildEmbyPath() = %v, want %v", result, tt.expectedPath)
			}
		})
	}
}
