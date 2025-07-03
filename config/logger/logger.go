package logger

import (
	"WebAppAnalyzer/config/env"
	"github.com/sirupsen/logrus"
)

type Logger struct {
	*logrus.Logger
}

func NewLogger(config env.Config) *Logger {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "timestamp",
			logrus.FieldKeyLevel: "level",
			logrus.FieldKeyMsg:   "message",
		},
	})
	if config.LogLevel == "debug" {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}
	logger.AddHook(&ApplicationHook{})

	return &Logger{logger}
}

type ApplicationHook struct{}

func (h *ApplicationHook) Fire(entry *logrus.Entry) error {
	entry.Data["level"] = entry.Level.String()
	entry.Data["application"] = "web-app-analyzer"
	entry.Data["version"] = "1.0.0"
	return nil
}

func (h *ApplicationHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (l *Logger) WithField(key string, value interface{}) *logrus.Entry {
	return l.Logger.WithField(key, value)
}
func (l *Logger) WithFields(fields logrus.Fields) *logrus.Entry {
	return l.Logger.WithFields(fields)
}
func (l *Logger) Info(args ...interface{}) {
	l.Logger.Info(args...)
}

func (l *Logger) Debug(args ...interface{}) {
	l.Logger.Debug(args...)
}

func (l *Logger) Warn(args ...interface{}) {
	l.Logger.Warn(args...)
}
func (l *Logger) Error(args ...interface{}) {
	l.Logger.Error(args...)
}
func (l *Logger) Fatal(args ...interface{}) {
	l.Logger.Fatal(args...)
}

func (l *Logger) WithRequest(method, path, remoteAddr string) *logrus.Entry {
	return l.WithFields(logrus.Fields{
		"method":      method,
		"path":        path,
		"remote_addr": remoteAddr,
	})
}
