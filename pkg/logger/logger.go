package logger

import (
	"os"

	"github.com/sirupsen/logrus"
)

type Logger struct {
	log *logrus.Logger
}

// NewLogger initializes a new instance of Logger
func NewLogger() *Logger {
	log := logrus.New()

	log.SetLevel(logrus.DebugLevel)

	// Set formatter to JSON if you want structured logs
	log.SetFormatter(&logrus.JSONFormatter{})

	// Log to stdout
	log.SetOutput(os.Stdout)

	return &Logger{log: log}
}

// Info logs an info-level message
func (l *Logger) Info(message string, fields map[string]interface{}) {
	l.log.WithFields(logrus.Fields(fields)).Info(message)
}

// Error logs an error-level message and sends it to Sentry if configured
func (l *Logger) Error(message string, fields map[string]interface{}, err error) {
	l.log.WithFields(logrus.Fields(fields)).Error(message)
}

// Debug logs a debug-level message
func (l *Logger) Debug(message string, fields map[string]interface{}) {
	l.log.WithFields(logrus.Fields(fields)).Debug(message)
}
