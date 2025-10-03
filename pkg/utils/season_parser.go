package utils

import (
	"fmt"
	"strconv"
	"strings"
)

// ExtractSeason 从路径或文件名中提取季度信息
// 返回格式化的季度字符串，如 "S01", "S02"
func ExtractSeason(path string) string {
	if path == "" {
		return "S01"
	}

	pathLower := strings.ToLower(path)

	// 模式1: S01, S02 格式
	if matches := SeasonPattern.FindStringSubmatch(pathLower); len(matches) > 1 {
		seasonNum, _ := strconv.Atoi(matches[1])
		return FormatSeason(seasonNum)
	}

	// 模式2: 第X季, 第X季（中文）
	if matches := ChineseSeasonPattern.FindStringSubmatch(path); len(matches) > 1 {
		seasonNum := ChineseToNumber(matches[1])
		if seasonNum > 0 {
			return FormatSeason(seasonNum)
		}
	}

	// 模式3: Season 1, Season 2
	if matches := SeasonEnglishPattern.FindStringSubmatch(pathLower); len(matches) > 1 {
		seasonNum, _ := strconv.Atoi(matches[1])
		return FormatSeason(seasonNum)
	}

	return "S01" // 默认第一季
}

// ExtractSeasonNumber 从路径中提取季度数字
// 返回整数，如果未找到返回 0
func ExtractSeasonNumber(path string) int {
	if path == "" {
		return 0
	}

	pathLower := strings.ToLower(path)

	// 模式1: S01, S02 格式
	if matches := SeasonPattern.FindStringSubmatch(pathLower); len(matches) > 1 {
		if num, err := strconv.Atoi(matches[1]); err == nil {
			return num
		}
	}

	// 模式2: 第X季（中文）
	if matches := ChineseSeasonPattern.FindStringSubmatch(path); len(matches) > 1 {
		return ChineseToNumber(matches[1])
	}

	// 模式3: Season 1, Season 2
	if matches := SeasonEnglishPattern.FindStringSubmatch(pathLower); len(matches) > 1 {
		if num, err := strconv.Atoi(matches[1]); err == nil {
			return num
		}
	}

	return 0
}

// FormatSeason 将季度数字格式化为标准格式 "S01", "S02"
func FormatSeason(seasonNum int) string {
	if seasonNum <= 0 {
		return "S01"
	}
	if seasonNum < 10 {
		return fmt.Sprintf("S0%d", seasonNum)
	}
	return fmt.Sprintf("S%d", seasonNum)
}

// IsSeasonDirectory 检查目录名是否为季度目录
func IsSeasonDirectory(dirName string) bool {
	if dirName == "" {
		return false
	}

	lowerDir := strings.ToLower(dirName)

	// 检查是否匹配季度模式
	patterns := []bool{
		SeasonStrictPattern.MatchString(lowerDir),                          // s1, s01, season 1
		ChineseSeasonPattern.MatchString(dirName),                          // 第1季, 第一季
		len(lowerDir) <= 4 && SeasonPattern.MatchString(lowerDir),         // s1, s01 (短格式)
	}

	for _, matched := range patterns {
		if matched {
			return true
		}
	}

	return false
}
