package logger

import (
	"log"
	"os"
)

type Logger struct {
	*log.Logger
}

var defaultLogger *Logger

func init() {
	defaultLogger = New()
}

func New() *Logger {
	return &Logger{
		Logger: log.New(os.Stdout, "[ALIST-ARIA2] ", log.LstdFlags|log.Lshortfile),
	}
}

func (l *Logger) Info(v ...interface{}) {
	l.SetPrefix("[ALIST-ARIA2] [INFO] ")
	l.Println(v...)
}

func (l *Logger) Error(v ...interface{}) {
	l.SetPrefix("[ALIST-ARIA2] [ERROR] ")
	l.Println(v...)
}

func (l *Logger) Debug(v ...interface{}) {
	l.SetPrefix("[ALIST-ARIA2] [DEBUG] ")
	l.Println(v...)
}

func (l *Logger) Warn(v ...interface{}) {
	l.SetPrefix("[ALIST-ARIA2] [WARN] ")
	l.Println(v...)
}

// 全局函数
func Info(v ...interface{}) {
	defaultLogger.Info(v...)
}

func Error(v ...interface{}) {
	defaultLogger.Error(v...)
}

func Debug(v ...interface{}) {
	defaultLogger.Debug(v...)
}

func Warn(v ...interface{}) {
	defaultLogger.Warn(v...)
}
