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
