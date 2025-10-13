package strutil

import (
	"testing"
)

func TestCleanShowName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "带网站水印的电影名",
			input:    "【高清影视之家发布 www.BBQDDQ.com】猫和老鼠：星盘奇缘[国语配音+中文字幕].Tom.and.Jerry.Forbidden.Compass.2025.2160p.WEB-DL.H265.HDR.DDP5.1-QuickIO",
			expected: "猫和老鼠星盘奇缘",
		},
		{
			name:     "带方括号水印",
			input:    "[www.example.com]电影名称.Movie.Name.2024.1080p",
			expected: "电影名称",
		},
		{
			name:     "普通中文名",
			input:    "流浪地球2",
			expected: "流浪地球2",
		},
		{
			name:     "带年份和括号",
			input:    "长津湖（2021）[完整版]",
			expected: "长津湖",
		},
		{
			name:     "带季度信息",
			input:    "庆余年.第二季.2024.全36集",
			expected: "庆余年",
		},
		{
			name:     "带S02格式季度信息",
			input:    "庆余年.S02.2024.1080p",
			expected: "庆余年",
		},
		{
			name:     "带Season 2格式",
			input:    "权力的游戏.Season 2.Complete",
			expected: "权力的游戏",
		},
		{
			name:     "英文名.中文名格式",
			input:    "Avatar.The.Way.of.Water.阿凡达：水之道",
			expected: "阿凡达水之道",
		},
		{
			name:     "带发布组信息",
			input:    "电影名称.2024.1080p.WEB-DL.H264-GroupName",
			expected: "电影名称",
		},
		{
			name:     "空字符串",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CleanShowName(tt.input)
			if result != tt.expected {
				t.Errorf("CleanShowName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCleanShowName_PreservesValidNames(t *testing.T) {
	validNames := []string{
		"流浪地球",
		"长津湖",
		"你好李焕英",
		"哪吒之魔童降世",
	}

	for _, name := range validNames {
		result := CleanShowName(name)
		if result != name {
			t.Errorf("CleanShowName(%q) = %q, should preserve valid name", name, result)
		}
	}
}
