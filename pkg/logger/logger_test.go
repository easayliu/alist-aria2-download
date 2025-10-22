package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogLevels(t *testing.T) {
	opts := Options{
		Level:    "debug",
		Output:   "console",
		Colorize: false,
	}

	if err := Init(opts); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	Debug("debug message", "key", "value")
	Info("info message", "key", "value")
	Warn("warn message", "key", "value")
	Error("error message", "key", "value")
}

func TestSetLevel(t *testing.T) {
	opts := Options{
		Level:    "info",
		Output:   "console",
		Colorize: false,
	}

	if err := Init(opts); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if err := SetLevel("error"); err != nil {
		t.Fatalf("SetLevel failed: %v", err)
	}

	Info("should not appear")
	Error("should appear")
}

func TestFileOutput(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	opts := Options{
		Level:    "debug",
		Output:   "file",
		Format:   "text",
		FilePath: logPath,
	}

	if err := Init(opts); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	Info("test message", "key", "value")

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Fatalf("Log file was not created")
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "test message") {
		t.Fatalf("Log content does not contain expected message")
	}
}

func TestJSONOutput(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.json")

	opts := Options{
		Level:    "info",
		Output:   "file",
		Format:   "json",
		FilePath: logPath,
	}

	if err := Init(opts); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	Info("test message", "key", "value")

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), `"msg":"test message"`) {
		t.Fatalf("JSON log does not contain expected message")
	}

	if !strings.Contains(string(content), `"key":"value"`) {
		t.Fatalf("JSON log does not contain expected key-value pair")
	}
}

func TestBothOutput(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	opts := Options{
		Level:    "info",
		Output:   "both",
		Format:   "json",
		FilePath: logPath,
		Colorize: true,
	}

	if err := Init(opts); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	Info("test message", "key", "value")

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Fatalf("Log file was not created")
	}
}

func TestInitDefault(t *testing.T) {
	defaultLogger = nil
	Info("test default init")

	if defaultLogger == nil {
		t.Fatal("Default logger was not initialized")
	}
}

func BenchmarkLogger(b *testing.B) {
	opts := Options{
		Level:    "info",
		Output:   "console",
		Colorize: false,
	}

	if err := Init(opts); err != nil {
		b.Fatalf("Init failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Info("benchmark message", "key", "value", "count", i)
	}
}

func BenchmarkLoggerWithColor(b *testing.B) {
	opts := Options{
		Level:    "info",
		Output:   "console",
		Colorize: true,
	}

	if err := Init(opts); err != nil {
		b.Fatalf("Init failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Info("benchmark message", "key", "value", "count", i)
	}
}
