package strutil

import "strconv"

// ChineseToNumber 将中文数字或阿拉伯数字字符串转换为整数
// 支持：一二三...九十、阿拉伯数字、组合形式（如十一、二十三）
func ChineseToNumber(str string) int {
	if str == "" {
		return 0
	}

	// 先尝试直接转换阿拉伯数字
	if num, err := strconv.Atoi(str); err == nil {
		return num
	}

	// 中文数字映射表
	chineseNumbers := map[string]int{
		"零": 0, "一": 1, "二": 2, "三": 3, "四": 4,
		"五": 5, "六": 6, "七": 7, "八": 8, "九": 9,
		"十": 10,
	}

	// 先尝试直接查表
	if num, ok := chineseNumbers[str]; ok {
		return num
	}

	// 转换为rune数组处理多字符
	runes := []rune(str)
	if len(runes) == 0 {
		return 0
	}

	// 处理 "十X" 格式（十一、十二...十九）
	if len(runes) >= 2 && string(runes[0]) == "十" {
		if num, ok := chineseNumbers[string(runes[1])]; ok {
			return 10 + num
		}
	}

	// 处理 "X十" 格式（二十、三十...）
	if len(runes) == 2 {
		tens := chineseNumbers[string(runes[0])]
		ones := chineseNumbers[string(runes[1])]

		// X十Y 格式（二十一、三十五...）
		if tens > 0 && ones == 10 {
			return tens * 10
		}
	}

	// 处理 "X十Y" 格式（二十一、三十五...）
	if len(runes) == 3 && string(runes[1]) == "十" {
		tens := chineseNumbers[string(runes[0])]
		ones := chineseNumbers[string(runes[2])]
		if tens > 0 && ones > 0 {
			return tens*10 + ones
		}
	}

	return 0
}
