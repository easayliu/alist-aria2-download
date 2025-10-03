package services

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
)

func TestDirectoryManager_EnsureDirectory(t *testing.T) {
	// 创建临时目录用于测试
	tmpDir := t.TempDir()

	autoCreate := true
	validatePerms := true
	checkDiskSpace := true

	cfg := &config.Config{
		Download: config.DownloadConfig{
			PathConfig: config.PathConfig{
				AutoCreateDir:       &autoCreate,
				ValidatePermissions: &validatePerms,
				CheckDiskSpace:      &checkDiskSpace,
			},
		},
	}
	manager := NewDirectoryManager(cfg)

	tests := []struct {
		name    string
		path    string
		setup   func() error
		wantErr bool
	}{
		{
			name:    "创建新目录",
			path:    filepath.Join(tmpDir, "test_dir"),
			setup:   func() error { return nil },
			wantErr: false,
		},
		{
			name: "目录已存在",
			path: filepath.Join(tmpDir, "existing_dir"),
			setup: func() error {
				return os.MkdirAll(filepath.Join(tmpDir, "existing_dir"), 0755)
			},
			wantErr: false,
		},
		{
			name: "路径是文件而非目录",
			path: filepath.Join(tmpDir, "test_file"),
			setup: func() error {
				return os.WriteFile(filepath.Join(tmpDir, "test_file"), []byte("test"), 0644)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				if err := tt.setup(); err != nil {
					t.Fatalf("Setup failed: %v", err)
				}
			}

			err := manager.EnsureDirectory(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("EnsureDirectory() error = %v, wantErr %v", err, tt.wantErr)
			}

			// 验证目录存在（如果不期望错误）
			if !tt.wantErr {
				if _, err := os.Stat(tt.path); os.IsNotExist(err) {
					t.Errorf("目录未创建: %s", tt.path)
				}
			}
		})
	}
}

func TestDirectoryManager_EnsureDirectory_NoAutoCreate(t *testing.T) {
	tmpDir := t.TempDir()

	autoCreate := false
	cfg := &config.Config{
		Download: config.DownloadConfig{
			PathConfig: config.PathConfig{
				AutoCreateDir: &autoCreate,
			},
		},
	}
	manager := NewDirectoryManager(cfg)

	testPath := filepath.Join(tmpDir, "no_auto_create")

	err := manager.EnsureDirectory(testPath)
	if err == nil {
		t.Error("期望错误（未启用自动创建），但成功了")
	}

	// 验证目录确实未创建
	if _, err := os.Stat(testPath); !os.IsNotExist(err) {
		t.Error("目录不应该被创建")
	}
}

func TestDirectoryManager_CheckDiskSpace(t *testing.T) {
	tmpDir := t.TempDir()

	checkDiskSpace := true
	cfg := &config.Config{
		Download: config.DownloadConfig{
			PathConfig: config.PathConfig{
				CheckDiskSpace: &checkDiskSpace,
			},
		},
	}
	manager := NewDirectoryManager(cfg)

	tests := []struct {
		name          string
		requiredBytes int64
		wantErr       bool
	}{
		{
			name:          "空间充足 - 1MB",
			requiredBytes: 1024 * 1024,
			wantErr:       false,
		},
		{
			name:          "空间充足 - 100MB",
			requiredBytes: 100 * 1024 * 1024,
			wantErr:       false,
		},
		{
			name:          "极大空间需求",
			requiredBytes: 1000 * 1024 * 1024 * 1024 * 1024, // 1PB
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.CheckDiskSpace(tmpDir, tt.requiredBytes)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckDiskSpace() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDirectoryManager_Cache(t *testing.T) {
	tmpDir := t.TempDir()

	autoCreate := true
	cfg := &config.Config{
		Download: config.DownloadConfig{
			PathConfig: config.PathConfig{
				AutoCreateDir: &autoCreate,
			},
		},
	}
	manager := NewDirectoryManager(cfg)

	testPath := filepath.Join(tmpDir, "cache_test")

	// 第一次调用 - 应该创建目录并加入缓存
	err := manager.EnsureDirectory(testPath)
	if err != nil {
		t.Fatalf("第一次调用失败: %v", err)
	}

	// 验证缓存大小
	if size := manager.GetCacheSize(); size != 1 {
		t.Errorf("缓存大小应该为1，实际为 %d", size)
	}

	// 第二次调用 - 应该直接从缓存返回
	err = manager.EnsureDirectory(testPath)
	if err != nil {
		t.Fatalf("第二次调用失败: %v", err)
	}

	// 清空缓存
	manager.ClearCache()
	if size := manager.GetCacheSize(); size != 0 {
		t.Errorf("清空后缓存大小应该为0，实际为 %d", size)
	}
}

func TestDirectoryManager_EnsureParentDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	autoCreate := true
	cfg := &config.Config{
		Download: config.DownloadConfig{
			PathConfig: config.PathConfig{
				AutoCreateDir: &autoCreate,
			},
		},
	}
	manager := NewDirectoryManager(cfg)

	// 测试文件路径（需要创建父目录）
	filePath := filepath.Join(tmpDir, "parent_test", "subdir", "file.txt")

	err := manager.EnsureParentDirectory(filePath)
	if err != nil {
		t.Fatalf("EnsureParentDirectory() 失败: %v", err)
	}

	// 验证父目录已创建
	parentDir := filepath.Dir(filePath)
	if _, err := os.Stat(parentDir); os.IsNotExist(err) {
		t.Errorf("父目录未创建: %s", parentDir)
	}
}

func TestDirectoryManager_ValidateDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// 创建一个测试目录
	testDir := filepath.Join(tmpDir, "validate_test")
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("创建测试目录失败: %v", err)
	}

	// 创建一个测试文件
	testFile := filepath.Join(tmpDir, "test_file.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
	}

	validatePerms := true
	cfg := &config.Config{
		Download: config.DownloadConfig{
			PathConfig: config.PathConfig{
				ValidatePermissions: &validatePerms,
			},
		},
	}
	manager := NewDirectoryManager(cfg)

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "有效目录",
			path:    testDir,
			wantErr: false,
		},
		{
			name:    "目录不存在",
			path:    filepath.Join(tmpDir, "nonexistent"),
			wantErr: true,
		},
		{
			name:    "路径是文件",
			path:    testFile,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.ValidateDirectory(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDirectory() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_formatSize(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{
			name:  "字节",
			bytes: 100,
			want:  "100 B",
		},
		{
			name:  "KB",
			bytes: 1024,
			want:  "1.0 KB",
		},
		{
			name:  "MB",
			bytes: 1024 * 1024,
			want:  "1.0 MB",
		},
		{
			name:  "GB",
			bytes: 1024 * 1024 * 1024,
			want:  "1.0 GB",
		},
		{
			name:  "TB",
			bytes: 1024 * 1024 * 1024 * 1024,
			want:  "1.0 TB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatSize(tt.bytes); got != tt.want {
				t.Errorf("formatSize() = %v, want %v", got, tt.want)
			}
		})
	}
}
