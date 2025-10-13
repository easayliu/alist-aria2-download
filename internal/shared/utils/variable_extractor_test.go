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

	file := contracts.FileResponse{
		Name: "【高清影视之家发布 www.BBQDDQ.com】猫和老鼠：星盘奇缘[国语配音+中文字幕].Tom.and.Jerry.Forbidden.Compass.2025.2160p.WEB-DL.H265.HDR.DDP5.1-QuickIO.mkv",
		Path: "/data/来自：分享/movies/【高清影视之家发布 www.BBQDDQ.com】猫和老鼠：星盘奇缘[国语配音+中文字幕].Tom.and.Jerry.Forbidden.Compass.2025.2160p.WEB-DL.H265.HDR.DDP5.1-QuickIO.mkv",
	}

	vars := extractor.ExtractVariables(file, "/downloads")

	if vars["category"] != "movie" {
		t.Errorf("category = %q, want %q", vars["category"], "movie")
	}

	if vars["title"] != "猫和老鼠星盘奇缘" {
		t.Errorf("title = %q, want %q", vars["title"], "猫和老鼠星盘奇缘")
	}

	t.Logf("✅ 变量提取成功: category=%s, title=%s", vars["category"], vars["title"])
}
