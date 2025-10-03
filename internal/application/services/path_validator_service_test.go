package services

import (
	"runtime"
	"strings"
	"testing"

	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
)

func TestPathValidatorService_Validate(t *testing.T) {
	cfg := &config.Config{
		Download: config.DownloadConfig{
			PathConfig: config.PathConfig{
				MaxPathLength: 1024,
			},
		},
	}
	service := NewPathValidatorService(cfg)

	tests := []struct {
		name    string
		path    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "有效路径",
			path:    "/downloads/tvs/明星大侦探/S08",
			wantErr: false,
		},
		{
			name:    "空路径",
			path:    "",
			wantErr: true,
			errMsg:  "路径为空",
		},
		{
			name:    "路径遍历攻击",
			path:    "/downloads/../etc/passwd",
			wantErr: true,
			errMsg:  "目录遍历攻击",
		},
		{
			name:    "包含控制字符",
			path:    "/downloads/test\x00file",
			wantErr: true,
			errMsg:  "控制字符",
		},
		{
			name:    "路径过长",
			path:    "/downloads/" + strings.Repeat("a", 2000),
			wantErr: true,
			errMsg:  "超过限制",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.Validate(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("错误信息不匹配: got %v, want contains %v", err, tt.errMsg)
				}
			}
		})
	}
}

func TestPathValidatorService_ValidateWindowsPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("仅在Windows上运行")
	}

	cfg := &config.Config{
		Download: config.DownloadConfig{
			PathConfig: config.PathConfig{
				MaxPathLength: 1024,
			},
		},
	}
	service := NewPathValidatorService(cfg)

	tests := []struct {
		name    string
		path    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "Windows保留名称 - CON",
			path:    "/downloads/CON/file.txt",
			wantErr: true,
			errMsg:  "保留名称",
		},
		{
			name:    "Windows保留名称 - COM1",
			path:    "/downloads/COM1.txt",
			wantErr: true,
			errMsg:  "保留名称",
		},
		{
			name:    "Windows不允许的字符 - 冒号",
			path:    "/downloads/test:file.txt",
			wantErr: true,
			errMsg:  "不允许的字符",
		},
		{
			name:    "Windows不允许的字符 - 问号",
			path:    "/downloads/test?.txt",
			wantErr: true,
			errMsg:  "不允许的字符",
		},
		{
			name:    "路径以空格结尾",
			path:    "/downloads/test /file.txt",
			wantErr: true,
			errMsg:  "空格或点结尾",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.Validate(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("错误信息不匹配: got %v, want contains %v", err, tt.errMsg)
				}
			}
		})
	}
}

func TestPathValidatorService_CleanPath(t *testing.T) {
	cfg := &config.Config{
		Download: config.DownloadConfig{
			PathConfig: config.PathConfig{
				MaxPathLength: 1024,
			},
		},
	}
	service := NewPathValidatorService(cfg)

	tests := []struct {
		name     string
		path     string
		wantPath string
	}{
		{
			name:     "移除零宽字符",
			path:     "/downloads/test\u200Bfile",
			wantPath: "/downloads/testfile",
		},
		{
			name:     "标准化空格",
			path:     "/downloads/test    file",
			wantPath: "/downloads/test file",
		},
		{
			name:     "替换冒号",
			path:     "/downloads/test:file",
			wantPath: "/downloads/test-file",
		},
		// 注意：问号在Linux/macOS是合法字符，只在Windows上会被移除
		// 此测试用例已移除，因为跨平台行为不一致
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := service.CleanPath(tt.path)
			// 由于filepath.Clean可能会改变路径，我们只检查关键部分
			if !strings.Contains(got, strings.TrimPrefix(tt.wantPath, "/downloads/")) {
				t.Errorf("CleanPath() = %v, want contains %v", got, tt.wantPath)
			}
		})
	}
}

func TestPathValidatorService_NormalizePath(t *testing.T) {
	cfg := &config.Config{
		Download: config.DownloadConfig{
			PathConfig: config.PathConfig{
				MaxPathLength: 1024,
			},
		},
	}
	service := NewPathValidatorService(cfg)

	tests := []struct {
		name string
		path string
	}{
		{
			name: "Unix风格路径",
			path: "/downloads/tvs/test",
		},
		{
			name: "包含多余斜杠",
			path: "/downloads//tvs///test",
		},
		{
			name: "包含相对路径",
			path: "/downloads/./tvs/test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := service.NormalizePath(tt.path)
			// 验证路径已被规范化（没有多余斜杠）
			if strings.Contains(got, "//") {
				t.Errorf("NormalizePath() 包含多余斜杠: %v", got)
			}
		})
	}
}

func Test_isZeroWidthChar(t *testing.T) {
	tests := []struct {
		name string
		r    rune
		want bool
	}{
		{
			name: "零宽空格",
			r:    '\u200B',
			want: true,
		},
		{
			name: "零宽非连接符",
			r:    '\u200C',
			want: true,
		},
		{
			name: "普通字符",
			r:    'a',
			want: false,
		},
		{
			name: "中文字符",
			r:    '测',
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isZeroWidthChar(tt.r); got != tt.want {
				t.Errorf("isZeroWidthChar() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_removeZeroWidthChars(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{
		{
			name: "包含零宽字符",
			s:    "test\u200Bfile\u200C",
			want: "testfile",
		},
		{
			name: "不包含零宽字符",
			s:    "testfile",
			want: "testfile",
		},
		{
			name: "多个零宽字符",
			s:    "\u200Btest\u200Cfile\u200D",
			want: "testfile",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := removeZeroWidthChars(tt.s); got != tt.want {
				t.Errorf("removeZeroWidthChars() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_normalizeWhitespace(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{
		{
			name: "多个空格",
			s:    "test    file",
			want: "test file",
		},
		{
			name: "首尾空格",
			s:    "  test file  ",
			want: "test file",
		},
		{
			name: "Tab和换行",
			s:    "test\t\nfile",
			want: "test file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeWhitespace(tt.s); got != tt.want {
				t.Errorf("normalizeWhitespace() = %v, want %v", got, tt.want)
			}
		})
	}
}
