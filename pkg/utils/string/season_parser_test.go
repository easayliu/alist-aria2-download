package strutil

import (
	"testing"
)

func TestExtractSeasonNumber(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "第二季",
			input:    "庆余年.第二季.2024.全36集",
			expected: 2,
		},
		{
			name:     "第一季",
			input:    "庆余年.第一季.2023",
			expected: 1,
		},
		{
			name:     "第十季",
			input:    "老友记.第十季",
			expected: 10,
		},
		{
			name:     "S02格式",
			input:    "庆余年.S02.2024",
			expected: 2,
		},
		{
			name:     "S1格式",
			input:    "庆余年.S1",
			expected: 1,
		},
		{
			name:     "Season 2格式",
			input:    "Game.of.Thrones.Season 2",
			expected: 2,
		},
		{
			name:     "无季度信息",
			input:    "电影名称.2024",
			expected: 0,
		},
		{
			name:     "第3季（数字）",
			input:    "第3季",
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractSeasonNumber(tt.input)
			if result != tt.expected {
				t.Errorf("ExtractSeasonNumber(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatSeason(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{1, "S01"},
		{2, "S02"},
		{10, "S10"},
		{15, "S15"},
		{0, "S01"},
		{-1, "S01"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatSeason(tt.input)
			if result != tt.expected {
				t.Errorf("FormatSeason(%d) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
