package logger

import (
	"log"
	"os"
)

type Logger struct {
	*log.Logger
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