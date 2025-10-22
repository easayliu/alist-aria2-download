package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

type Logger struct {
	slogger *slog.Logger
	level   *slog.LevelVar
}

var defaultLogger *Logger

const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorGray   = "\033[90m"
	ColorGreen  = "\033[32m"
)

type Options struct {
	Level     string
	Output    string
	Format    string
	FilePath  string
	Colorize  bool
	AddSource bool
}

func Init(opts Options) error {
	level := parseLevel(opts.Level)
	levelVar := &slog.LevelVar{}
	levelVar.Set(level)

	var handlers []slog.Handler

	handlerOpts := &slog.HandlerOptions{
		Level:     levelVar,
		AddSource: opts.AddSource,
	}

	switch opts.Output {
	case "console", "":
		if opts.Colorize {
			handlers = append(handlers, newColorHandler(os.Stdout, handlerOpts))
		} else {
			handlers = append(handlers, newTextHandler(os.Stdout, handlerOpts))
		}

	case "file":
		file, err := openLogFile(opts.FilePath)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}

		if opts.Format == "json" {
			handlers = append(handlers, slog.NewJSONHandler(file, handlerOpts))
		} else {
			handlers = append(handlers, newTextHandler(file, handlerOpts))
		}

	case "both":
		if opts.Colorize {
			handlers = append(handlers, newColorHandler(os.Stdout, handlerOpts))
		} else {
			handlers = append(handlers, newTextHandler(os.Stdout, handlerOpts))
		}

		file, err := openLogFile(opts.FilePath)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}

		if opts.Format == "json" {
			handlers = append(handlers, slog.NewJSONHandler(file, handlerOpts))
		} else {
			handlers = append(handlers, newTextHandler(file, handlerOpts))
		}
	default:
		return fmt.Errorf("invalid output mode: %s", opts.Output)
	}

	var handler slog.Handler
	if len(handlers) == 1 {
		handler = handlers[0]
	} else {
		handler = newMultiHandler(handlers...)
	}

	defaultLogger = &Logger{
		slogger: slog.New(handler),
		level:   levelVar,
	}

	return nil
}

func openLogFile(path string) (*os.File, error) {
	if path == "" {
		return nil, fmt.Errorf("log file path is empty")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return file, nil
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func SetLevel(level string) error {
	if defaultLogger == nil {
		return fmt.Errorf("logger not initialized")
	}
	defaultLogger.level.Set(parseLevel(level))
	return nil
}

func Sync() error {
	return nil
}

func Debug(msg string, args ...any) {
	if defaultLogger == nil {
		initDefault()
	}
	defaultLogger.slogger.Debug(msg, args...)
}

func Info(msg string, args ...any) {
	if defaultLogger == nil {
		initDefault()
	}
	defaultLogger.slogger.Info(msg, args...)
}

func Warn(msg string, args ...any) {
	if defaultLogger == nil {
		initDefault()
	}
	defaultLogger.slogger.Warn(msg, args...)
}

func Error(msg string, args ...any) {
	if defaultLogger == nil {
		initDefault()
	}
	defaultLogger.slogger.Error(msg, args...)
}

func initDefault() {
	Init(Options{
		Level:    "info",
		Output:   "console",
		Colorize: true,
	})
}

func newTextHandler(w io.Writer, opts *slog.HandlerOptions) slog.Handler {
	return slog.NewTextHandler(w, opts)
}

func newColorHandler(w io.Writer, opts *slog.HandlerOptions) slog.Handler {
	return &colorHandler{
		Handler: slog.NewTextHandler(w, opts),
		w:       w,
	}
}

type colorHandler struct {
	slog.Handler
	w io.Writer
}

func (h *colorHandler) Handle(ctx context.Context, r slog.Record) error {
	level := r.Level.String()
	color := getColor(r.Level)

	buf := make([]byte, 0, 1024)

	timestamp := r.Time.Format("2006-01-02 15:04:05")
	buf = append(buf, []byte(ColorGray)...)
	buf = append(buf, []byte(timestamp)...)
	buf = append(buf, []byte(ColorReset)...)
	buf = append(buf, ' ')

	buf = append(buf, []byte(fmt.Sprintf("%s[%s]%s ", color, level, ColorReset))...)

	msg := r.Message
	buf = append(buf, []byte(msg)...)

	r.Attrs(func(a slog.Attr) bool {
		buf = append(buf, ' ')
		buf = append(buf, []byte(ColorGray)...)
		buf = append(buf, []byte(a.Key)...)
		buf = append(buf, '=')
		buf = append(buf, []byte(a.Value.String())...)
		buf = append(buf, []byte(ColorReset)...)
		return true
	})

	buf = append(buf, '\n')
	_, err := h.w.Write(buf)
	return err
}

func getColor(level slog.Level) string {
	switch level {
	case slog.LevelDebug:
		return ColorGray
	case slog.LevelInfo:
		return ColorBlue
	case slog.LevelWarn:
		return ColorYellow
	case slog.LevelError:
		return ColorRed
	default:
		return ColorReset
	}
}

type multiHandler struct {
	handlers []slog.Handler
}

func newMultiHandler(handlers ...slog.Handler) *multiHandler {
	return &multiHandler{handlers: handlers}
}

func (h *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, handler := range h.handlers {
		if err := handler.Handle(ctx, r); err != nil {
			return err
		}
	}
	return nil
}

func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithAttrs(attrs)
	}
	return &multiHandler{handlers: handlers}
}

func (h *multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithGroup(name)
	}
	return &multiHandler{handlers: handlers}
}
