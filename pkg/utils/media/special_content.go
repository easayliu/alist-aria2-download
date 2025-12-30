package media

import (
	"regexp"
	"strings"
)

// episodePattern 用于检测文件名是否包含明确的期/集信息
var episodePattern = regexp.MustCompile(`(?:第\s*\d+\s*[期集话話]|[Ss]\d+[Ee]\d+|[Ee]\d+)`)

// IsSpecialContent 检查文件名是否为特殊内容
// 特殊内容包括: 加更、花絮、预告、特辑、综艺衍生内容等
// 这些内容不适合用标准剧集命名规则处理
func IsSpecialContent(fileName string) bool {
	lowerFileName := strings.ToLower(fileName)

	// 如果文件名包含明确的期/集信息，先检查是否匹配"弱关键词"
	// 弱关键词只有在没有期/集信息时才认为是特殊内容
	hasEpisodeInfo := episodePattern.MatchString(fileName)

	for _, keyword := range SpecialContentKeywords {
		if strings.Contains(lowerFileName, keyword) {
			// 如果是弱关键词且文件名包含期/集信息，则不认为是特殊内容
			if hasEpisodeInfo && isWeakKeyword(keyword) {
				continue
			}
			return true
		}
	}
	return false
}

// isWeakKeyword 判断是否为弱关键词
// 弱关键词可能出现在正常剧集标题中，需要结合上下文判断
func isWeakKeyword(keyword string) bool {
	weakKeywords := map[string]bool{
		"收官": true, // "喜迎收官"等标题
		"回顾": true, // "回顾XX"可能是正常标题
		"精彩": true, // "精彩瞬间"可能是正常标题
	}
	return weakKeywords[keyword]
}

// SpecialContentKeywords 特殊内容关键词列表
var SpecialContentKeywords = []string{
	// 中文关键词
	"加更", "花絮", "预告", "片花", "幕后", "特辑",
	"番外", "访谈", "采访", "回顾", "精彩", "集锦", "合集",
	"首映", "特别企划", "收官", "先导",
	// 彩蛋相关（使用更精确的表达，避免误判如"找彩蛋"等剧情内容）
	"片尾彩蛋", "彩蛋视频", "隐藏彩蛋",
	// 综艺衍生内容
	"超前vlog", "超前营业", "陪看记", "母带放送", "惊喜母带",
	"独家记忆", "全员花絮", "制作特辑",
	// 英文关键词
	"vlog", "behind", "making",
	"trailer", "preview", "bonus", "extra", "special",
}

// SpinOffKeywords 衍生节目关键词（这些节目应该加后缀区分，而非跳过）
var SpinOffKeywords = []string{
	"跑男来了",    // 奔跑吧衍生
	"密室大逃脱加更", // 密室大逃脱衍生
	"来了加更版",   // 通用加更版
}

// DetectSpinOff 检测文件名中的衍生节目名称
// 返回衍生节目名称，如果不是衍生节目返回空字符串
func DetectSpinOff(fileName string) string {
	for _, keyword := range SpinOffKeywords {
		if strings.Contains(fileName, keyword) {
			return keyword
		}
	}
	return ""
}
