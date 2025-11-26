package logger

import (
	"testing"
)

func TestMaskToken(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "空字符串",
			input: "",
			want:  "",
		},
		{
			name:  "短token(<8字符)",
			input: "abc",
			want:  "***",
		},
		{
			name:  "短token(7字符)",
			input: "1234567",
			want:  "***",
		},
		{
			name:  "正好8字符",
			input: "12345678",
			want:  "12345678",
		},
		{
			name:  "长token(16字符)",
			input: "1234567890abcdef",
			want:  "1234********cdef",
		},
		{
			name:  "很长的token(32字符)",
			input: "12345678901234567890123456789012",
			want:  "1234************************9012",
		},
		{
			name:  "实际Bearer token",
			input: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			want:  "eyJh****************************VCJ9",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaskToken(tt.input)
			if got != tt.want {
				t.Errorf("MaskToken() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSanitizeValue(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value interface{}
		want  interface{}
	}{
		{
			name:  "普通字段不脱敏",
			key:   "username",
			value: "john_doe",
			want:  "john_doe",
		},
		{
			name:  "token字段脱敏",
			key:   "token",
			value: "1234567890abcdef",
			want:  "1234********cdef",
		},
		{
			name:  "access_token字段脱敏",
			key:   "access_token",
			value: "Bearer_1234567890abcdefghij",
			want:  "Bear*******************ghij", // 27-8=19个星号
		},
		{
			name:  "password字段脱敏",
			key:   "password",
			value: "myPassword123",
			want:  "myPa*****d123", // 13-8=5个星号
		},
		{
			name:  "api_key字段脱敏",
			key:   "api_key",
			value: "sk-abc123def456",
			want:  "sk-a*******f456", // 15-8=7个星号
		},
		{
			name:  "大小写不敏感-TOKEN",
			key:   "AUTH_TOKEN",
			value: "token123456789",
			want:  "toke******6789", // 14-8=6个星号
		},
		{
			name:  "非字符串token脱敏",
			key:   "token",
			value: 12345,
			want:  "***MASKED***",
		},
		{
			name:  "短密码脱敏",
			key:   "pwd",
			value: "123",
			want:  "***",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeValue(tt.key, tt.value)
			if got != tt.want {
				t.Errorf("SanitizeValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSanitizeArgs(t *testing.T) {
	tests := []struct {
		name string
		args []any
		want []any
	}{
		{
			name: "空参数",
			args: []any{},
			want: []any{},
		},
		{
			name: "无敏感信息",
			args: []any{"username", "john", "action", "login"},
			want: []any{"username", "john", "action", "login"},
		},
		{
			name: "包含token",
			args: []any{"username", "john", "token", "1234567890abcdef"},
			want: []any{"username", "john", "token", "1234********cdef"},
		},
		{
			name: "混合敏感和非敏感",
			args: []any{
				"user", "alice",
				"password", "myPassword123",
				"action", "create",
				"api_key", "sk-abc123def456",
			},
			want: []any{
				"user", "alice",
				"password", "myPa*****d123", // 13-8=5个星号
				"action", "create",
				"api_key", "sk-a*******f456", // 15-8=7个星号
			},
		},
		{
			name: "奇数参数(最后一个key无value)",
			args: []any{"user", "bob", "token"},
			want: []any{"user", "bob", "token"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeArgs(tt.args...)
			if len(got) != len(tt.want) {
				t.Errorf("SanitizeArgs() length = %v, want %v", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("SanitizeArgs()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestIsSensitiveKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want bool
	}{
		{"token", "token", true},
		{"access_token", "access_token", true},
		{"password", "password", true},
		{"user_password", "user_password", true},
		{"api_key", "api_key", true},
		{"apikey", "apikey", true},
		{"secret", "secret", true},
		{"username", "username", false},
		{"user_id", "user_id", false},
		{"action", "action", false},
		{"大写TOKEN", "AUTH_TOKEN", true},
		{"大写PASSWORD", "USER_PASSWORD", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSensitiveKey(tt.key)
			if got != tt.want {
				t.Errorf("IsSensitiveKey(%s) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestSafeFormat(t *testing.T) {
	tests := []struct {
		name   string
		format string
		args   []interface{}
		want   string
	}{
		{
			name:   "无敏感信息",
			format: "User %s logged in",
			args:   []interface{}{"alice"},
			want:   "User alice logged in",
		},
		{
			name:   "包含可能的token",
			format: "Token: %s",
			args:   []interface{}{"1234567890abcdef"},
			want:   "Token: 1234********cdef",
		},
		{
			name:   "多个参数",
			format: "User %s with token %s",
			args:   []interface{}{"bob", "token_1234567890abc"},
			want:   "User bob with token toke***********0abc", // 19-8=11个星号
		},
		{
			name:   "短字符串不脱敏",
			format: "Code: %s",
			args:   []interface{}{"123"},
			want:   "Code: 123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SafeFormat(tt.format, tt.args...)
			if got != tt.want {
				t.Errorf("SafeFormat() = %v, want %v", got, tt.want)
			}
		})
	}
}

// 基准测试
func BenchmarkMaskToken(b *testing.B) {
	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ"
	for i := 0; i < b.N; i++ {
		MaskToken(token)
	}
}

func BenchmarkSanitizeArgs(b *testing.B) {
	args := []any{
		"user", "alice",
		"token", "1234567890abcdef",
		"action", "login",
		"api_key", "sk-1234567890",
	}
	for i := 0; i < b.N; i++ {
		SanitizeArgs(args...)
	}
}
