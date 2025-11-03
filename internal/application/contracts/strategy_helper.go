package contracts

// ParseHybridStrategy 解析策略字符串为枚举
func ParseHybridStrategy(strategy string) HybridStrategy {
	switch strategy {
	case "llm_first":
		return LLMFirst
	case "llm_only":
		return LLMOnly
	case "tmdb_only":
		return TMDBOnly
	case "compare":
		return Compare
	case "tmdb_first", "":
		return TMDBFirst
	default:
		return TMDBFirst
	}
}
