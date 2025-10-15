package utils

import (
	"testing"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
)

func TestExtractMovieTitle(t *testing.T) {
	extractor := NewVariableExtractor()

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "电影路径-带网站水印",
			path:     "/data/来自：分享/movies/【高清影视之家发布 www.BBQDDQ.com】猫和老鼠：星盘奇缘[国语配音+中文字幕].Tom.and.Jerry.Forbidden.Compass.2025.2160p.WEB-DL.H265.HDR.DDP5.1-QuickIO",
			expected: "猫和老鼠星盘奇缘",
		},
		{
			name:     "电影路径-简单中文名",
			path:     "/data/movies/流浪地球2.2023.1080p",
			expected: "流浪地球2",
		},
		{
			name:     "电影路径-带括号",
			path:     "/data/movies/长津湖（2021）[完整版]",
			expected: "长津湖",
		},
		{
			name:     "电影路径-深层年份目录",
			path:     "/data/movies/2024/猫和老鼠/movie.mkv",
			expected: "猫和老鼠",
		},
		{
			name:     "电影路径-多层分类目录",
			path:     "/data/movies/华语/2024/流浪地球/movie.mkv",
			expected: "流浪地球",
		},
		{
			name:     "电影路径-地区分类",
			path:     "/data/movies/欧美/阿凡达/movie.mkv",
			expected: "阿凡达",
		},
		{
			name:     "电影路径-质量分类",
			path:     "/data/movies/4K/电影名/movie.mkv",
			expected: "电影名",
		},
		{
			name:     "电影路径-国产分类",
			path:     "/data/movies/国产/长津湖/movie.mkv",
			expected: "长津湖",
		},
		{
			name:     "电影路径-合集系列带网站水印",
			path:     "/data/来自：分享/movies/【高清影视之家发布 www.HDBTHD.com】黑衣人[共3部合集][中文字幕].Men.in.Black.1997-2012.BluRay.2160p.Atmos.TrueHD7.1.x265.10bit-DreamHD/黑衣人1.mkv",
			expected: "黑衣人",
		},
		{
			name:     "电影路径-无间道复杂音频格式",
			path:     "/data/来自：分享/movies/【首发于高清影视之家 www.BBQDDQ.com】无间道[共3部合集][国粤多音轨+中文字幕].Infernal.Affairs.Collection.2002-2003.2160p.BluRay.2Audio.DTS-HDMA5.1.HDR10.x265-DreamHD/无间道1.mkv",
			expected: "无间道",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractor.extractMovieTitle(tt.path)
			if result != tt.expected {
				t.Errorf("extractMovieTitle(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestCleanMovieTitle(t *testing.T) {
	extractor := NewVariableExtractor()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "带网站水印的完整标题",
			input:    "【高清影视之家发布 www.BBQDDQ.com】猫和老鼠：星盘奇缘[国语配音+中文字幕].Tom.and.Jerry.Forbidden.Compass.2025.2160p.WEB-DL.H265.HDR.DDP5.1-QuickIO",
			expected: "猫和老鼠星盘奇缘",
		},
		{
			name:     "简单电影名",
			input:    "流浪地球2",
			expected: "流浪地球2",
		},
		{
			name:     "带质量标记",
			input:    "阿凡达.2024.1080p.BluRay",
			expected: "阿凡达",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractor.cleanMovieTitle(tt.input)
			if result != tt.expected {
				t.Errorf("cleanMovieTitle(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractVariables_Movie(t *testing.T) {
	extractor := NewVariableExtractor()

	tests := []struct {
		name     string
		file     contracts.FileResponse
		category string
		title    string
	}{
		{
			name: "电影-带网站水印",
			file: contracts.FileResponse{
				Name: "【高清影视之家发布 www.BBQDDQ.com】猫和老鼠：星盘奇缘[国语配音+中文字幕].Tom.and.Jerry.Forbidden.Compass.2025.2160p.WEB-DL.H265.HDR.DDP5.1-QuickIO.mkv",
				Path: "/data/来自：分享/movies/【高清影视之家发布 www.BBQDDQ.com】猫和老鼠：星盘奇缘[国语配音+中文字幕].Tom.and.Jerry.Forbidden.Compass.2025.2160p.WEB-DL.H265.HDR.DDP5.1-QuickIO.mkv",
			},
			category: "movie",
			title:    "猫和老鼠星盘奇缘",
		},
		{
			name: "电影-合集系列",
			file: contracts.FileResponse{
				Name: "黑衣人1.mkv",
				Path: "/data/来自：分享/movies/【高清影视之家发布 www.HDBTHD.com】黑衣人[共3部合集][中文字幕].Men.in.Black.1997-2012.BluRay.2160p.Atmos.TrueHD7.1.x265.10bit-DreamHD/黑衣人1.mkv",
			},
			category: "movie",
			title:    "黑衣人",
		},
		{
			name: "电影-无间道复杂音频",
			file: contracts.FileResponse{
				Name: "无间道1.mkv",
				Path: "/data/来自：分享/movies/【首发于高清影视之家 www.BBQDDQ.com】无间道[共3部合集][国粤多音轨+中文字幕].Infernal.Affairs.Collection.2002-2003.2160p.BluRay.2Audio.DTS-HDMA5.1.HDR10.x265-DreamHD/无间道1.mkv",
			},
			category: "movie",
			title:    "无间道",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vars := extractor.ExtractVariables(tt.file, "/downloads")

			if vars["category"] != tt.category {
				t.Errorf("category = %q, want %q", vars["category"], tt.category)
			}

			if vars["title"] != tt.title {
				t.Errorf("title = %q, want %q", vars["title"], tt.title)
			}

			t.Logf("✅ 变量提取成功: category=%s, title=%s", vars["category"], vars["title"])
		})
	}
}

func TestExtractVariables_TVShow(t *testing.T) {
	extractor := NewVariableExtractor()

	tests := []struct {
		name     string
		file     contracts.FileResponse
		category string
		show     string
		season   string
	}{
		{
			name: "电视剧-tvs目录合集",
			file: contracts.FileResponse{
				Name: "A.Bite.of.China.2012.E07.BluRay.1080p.DTS.HDMA5.1.x265.10bit-DreamHD.mkv",
				Path: "/data/来自：分享/tvs/【高清影视之家首发 www.BBQDDQ.com】舌尖上的中国 第一季[共7部合集][国语音轨+中英字幕].A.Bite.of.China.2012.BluRay.1080p.DTS.HDMA5.1.x265.10bit-DreamHD/A.Bite.of.China.2012.E07.BluRay.1080p.DTS.HDMA5.1.x265.10bit-DreamHD.mkv",
			},
			category: "tv",
			show:     "舌尖上的中国",
			season:   "S01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vars := extractor.ExtractVariables(tt.file, "/downloads")

			if vars["category"] != tt.category {
				t.Errorf("category = %q, want %q", vars["category"], tt.category)
			}

			if vars["show"] != tt.show {
				t.Errorf("show = %q, want %q", vars["show"], tt.show)
			}

			if vars["season"] != tt.season {
				t.Errorf("season = %q, want %q", vars["season"], tt.season)
			}

			t.Logf("✅ 变量提取成功: category=%s, show=%s, season=%s", vars["category"], vars["show"], vars["season"])
		})
	}
}

func TestExtractShowName(t *testing.T) {
	extractor := NewVariableExtractor()

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "电视剧-标准路径",
			path:     "/data/tvs/庆余年/S02/E01.mkv",
			expected: "庆余年",
		},
		{
			name:     "电视剧-深层地区分类",
			path:     "/data/tvs/国产/庆余年/S02/E01.mkv",
			expected: "庆余年",
		},
		{
			name:     "电视剧-年份分类",
			path:     "/data/tvs/2024/庆余年/S02/E01.mkv",
			expected: "庆余年",
		},
		{
			name:     "电视剧-多层分类",
			path:     "/data/tvs/华语/2024/庆余年/S02/E01.mkv",
			expected: "庆余年",
		},
		{
			name:     "电视剧-目录名带网站水印",
			path:     "/data/tvs/【高清影视之家发布 www.BBQDDQ.com】猫和老鼠剧集/S01/E01.mkv",
			expected: "猫和老鼠剧集",
		},
		{
			name:     "电视剧-分类目录+网站水印",
			path:     "/data/tvs/2024/【高清影视之家发布 www...】某电视剧/S01/E01.mkv",
			expected: "某电视剧",
		},
		{
			name:     "电视剧-国产分类+网站水印",
			path:     "/data/tvs/国产/【高清影视之家发布】庆余年.第二季/S02/E01.mkv",
			expected: "庆余年",
		},
		{
			name:     "综艺-标准路径",
			path:     "/data/variety/向往的生活/20240628.mkv",
			expected: "向往的生活",
		},
		{
			name:     "电视剧-tvs目录下的合集",
			path:     "/data/来自：分享/tvs/【高清影视之家首发 www.BBQDDQ.com】舌尖上的中国 第一季[共7部合集][国语音轨+中英字幕].A.Bite.of.China.2012.BluRay.1080p.DTS.HDMA5.1.x265.10bit-DreamHD/A.Bite.of.China.2012.E07.BluRay.1080p.DTS.HDMA5.1.x265.10bit-DreamHD.mkv",
			expected: "舌尖上的中国",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractor.extractShowName(tt.path)
			if result != tt.expected {
				t.Errorf("extractShowName(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}
