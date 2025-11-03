package media

import "strings"

// IsSpecialContent 检查文件名是否为特殊内容
// 特殊内容包括: 加更、花絮、预告、特辑、综艺衍生内容等
// 这些内容不适合用标准剧集命名规则处理
func IsSpecialContent(fileName string) bool {
	lowerFileName := strings.ToLower(fileName)
	for _, keyword := range SpecialContentKeywords {
		if strings.Contains(lowerFileName, keyword) {
			return true
		}
	}
	return false
}

// SpecialContentKeywords 特殊内容关键词列表
var SpecialContentKeywords = []string{
	// 中文关键词
	"加更", "花絮", "预告", "片花", "彩蛋", "幕后", "特辑",
	"番外", "访谈", "采访", "回顾", "精彩", "集锦", "合集",
	"首映", "特别企划", "收官", "先导",
	// 综艺衍生内容
	"超前vlog", "超前营业", "陪看记", "母带放送", "惊喜母带",
	"独家记忆", "全员花絮", "制作特辑",
	// 英文关键词
	"vlog", "behind", "making",
	"trailer", "preview", "bonus", "extra", "special",
}
