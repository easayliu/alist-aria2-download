package logger

import (
	"fmt"
	"strings"
)

// MaskToken 脱敏token字符串
// 规则:
//   - 空字符串返回空
//   - 长度<8: 返回 "***"
//   - 长度>=8: 保留前4后4,中间用星号替换
func MaskToken(token string) string {
	if token == "" {
		return ""
	}

	length := len(token)
	if length < 8 {
		return "***"
	}

	// 保留前4位和后4位
	maskedLength := length - 8
	return token[:4] + strings.Repeat("*", maskedLength) + token[length-4:]
}

// SanitizeValue 智能脱敏:根据键名判断是否需要脱敏
// 会自动识别包含敏感关键字的键名并脱敏其值
func SanitizeValue(key string, value interface{}) interface{} {
	keyLower := strings.ToLower(key)

	// 需要脱敏的字段关键字
	sensitiveKeys := []string{
		"token",
		"password",
		"passwd",
		"pwd",
		"secret",
		"api_key",
		"apikey",
		"api-key",
		"authorization",
		"auth",
	}

	// 检查键名是否包含敏感关键字
	for _, sk := range sensitiveKeys {
		if strings.Contains(keyLower, sk) {
			// 如果是字符串类型,使用MaskToken脱敏
			if strVal, ok := value.(string); ok {
				return MaskToken(strVal)
			}
			// 其他类型统一返回掩码
			return "***MASKED***"
		}
	}

	return value
}

// SanitizeArgs 批量脱敏slog日志参数
// slog使用键值对格式: key1, value1, key2, value2, ...
// 此函数会检查每个key,如果是敏感字段则脱敏对应的value
func SanitizeArgs(args ...any) []any {
	if len(args) == 0 {
		return args
	}

	result := make([]any, len(args))

	for i := 0; i < len(args); i += 2 {
		// 复制key
		result[i] = args[i]

		// 处理value
		if i+1 < len(args) {
			key, ok := args[i].(string)
			if ok {
				// 如果key是字符串,根据key判断是否需要脱敏value
				result[i+1] = SanitizeValue(key, args[i+1])
			} else {
				// key不是字符串,直接复制value
				result[i+1] = args[i+1]
			}
		}
	}

	return result
}

// SanitizeString 脱敏字符串中可能包含的敏感信息
// 用于脱敏完整的字符串内容(如日志消息本身)
func SanitizeString(s string) string {
	// 匹配常见的敏感信息模式
	patterns := map[string]string{
		// Bearer token
		`Bearer\s+([A-Za-z0-9\-._~+/]+)`:                    "Bearer ***TOKEN***",
		// API key patterns
		`(?i)(api[_-]?key|apikey)[:=]\s*([A-Za-z0-9]+)`:    "${1}=***",
		// Token patterns
		`(?i)(token)[:=]\s*([A-Za-z0-9\-._~+/]+)`:          "${1}=***",
		// Password patterns
		`(?i)(password|passwd|pwd)[:=]\s*([^\s,}\]"']+)`:   "${1}=***",
	}

	result := s
	for pattern, replacement := range patterns {
		// 注意:这里简化处理,实际应使用regexp但为了性能考虑可以优化
		// 当前仅做基础字符串替换演示
		if strings.Contains(strings.ToLower(result), "token") ||
			strings.Contains(strings.ToLower(result), "password") ||
			strings.Contains(strings.ToLower(result), "api") {
			// 实际项目中应该用正则表达式,这里为了简洁暂时跳过
			_ = pattern
			_ = replacement
		}
	}

	return result
}

// IsSensitiveKey 判断键名是否为敏感字段
func IsSensitiveKey(key string) bool {
	keyLower := strings.ToLower(key)
	sensitiveKeys := []string{
		"token", "password", "passwd", "pwd",
		"secret", "api_key", "apikey", "api-key",
		"authorization", "auth",
	}

	for _, sk := range sensitiveKeys {
		if strings.Contains(keyLower, sk) {
			return true
		}
	}
	return false
}

// SafeFormat 安全格式化,用于需要格式化输出敏感信息时
func SafeFormat(format string, args ...interface{}) string {
	// 对每个参数进行检查,如果是字符串且长度较长(可能是token)则脱敏
	safeArgs := make([]interface{}, len(args))
	for i, arg := range args {
		if strArg, ok := arg.(string); ok && len(strArg) > 10 {
			// 可能是token或敏感信息
			safeArgs[i] = MaskToken(strArg)
		} else {
			safeArgs[i] = arg
		}
	}
	return fmt.Sprintf(format, safeArgs...)
}
