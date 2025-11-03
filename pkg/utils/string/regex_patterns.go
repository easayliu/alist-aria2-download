package strutil

import "regexp"

// 预编译的正则表达式模式，避免重复编译提升性能

var (
	// Season 相关模式
	SeasonPattern        = regexp.MustCompile(`[sS](\d{1,2})`)                    // S01, S1, s01
	SeasonPatternCI      = regexp.MustCompile(`(?i)(^|[/\s])s(\d{1,2})($|[/\s])`) // 不区分大小写
	SeasonEnglishPattern = regexp.MustCompile(`(?i)season[\s_-]?(\d+)`)           // Season 1, season1
	SeasonStrictPattern  = regexp.MustCompile(`^(?:s|season\s*)(\d{1,2})$`)       // 严格匹配
	ChineseSeasonPattern = regexp.MustCompile(`第([零一二三四五六七八九十百\d]+)季`)            // 第1季, 第一季

	// Episode 相关模式
	EpisodePattern        = regexp.MustCompile(`[eE](\d{1,3})`)          // E01, E1
	EpisodeEPPattern      = regexp.MustCompile(`(?i)ep[\s_-]?(\d{1,3})`) // EP01, ep1
	EpisodePatternCI      = regexp.MustCompile(`(?i)(^|[^A-Za-z])(E|EP)(\d{1,3})($|[^0-9])`)
	ChineseEpisodePattern = regexp.MustCompile(`第([零一二三四五六七八九十百\d]+)集`) // 第1集, 第一集

	// Season + Episode 组合模式
	SeasonEpisodePattern = regexp.MustCompile(`(?i)S(\d{1,2})E\d{1,3}`) // S01E01

	// 日期模式
	DatePattern = regexp.MustCompile(`\b20\d{6}\b`) // 20240101

	// 年份模式
	YearPattern = regexp.MustCompile(`[\(（\[]?(19\d{2}|20\d{2})[\)）\]]?`) // (2024), [2024]

	// 中文季度后缀
	ChineseSeasonSuffixPattern = regexp.MustCompile(`(?i)\s*第[\p{Han}\d]{1,4}季.*$`)

	// 空白符
	WhitespacePattern = regexp.MustCompile(`\s+`)
)
