package logging

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

const (
	tsFormat   = "2006-01-02T15:04:05.000Z07:00"
	fieldTime  = "timestamp"
	fieldLevel = "level"
	fieldMsg   = "message"
)

func fieldMap() logrus.FieldMap {
	return logrus.FieldMap{
		logrus.FieldKeyTime:  fieldTime,
		logrus.FieldKeyLevel: fieldLevel,
		logrus.FieldKeyMsg:   fieldMsg,
	}
}

// SetupLogger configures logrus with environment variables
func SetupLogger() *logrus.Logger {
	logger := logrus.New()

	logLevel := strings.ToLower(os.Getenv("LOG_LEVEL"))
	if logLevel == "" {
		logLevel = "info"
	}
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		logger.Warnf("Invalid log level '%s', using 'info'", logLevel)
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)

	logFormat := strings.ToLower(os.Getenv("LOG_FORMAT"))
	if logFormat == "" {
		logFormat = "json"
	}

	switch logFormat {
	case "json":
		logger.SetFormatter(&logrus.JSONFormatter{TimestampFormat: tsFormat, FieldMap: fieldMap()})
	case "text":
		logger.SetFormatter(&logrus.TextFormatter{TimestampFormat: tsFormat, FieldMap: fieldMap()})
	case "colored", "color":
		logger.SetFormatter(&logrus.TextFormatter{ForceColors: true, TimestampFormat: tsFormat, FieldMap: fieldMap()})
	default:
		logger.Warnf("Invalid log format '%s', using 'json'", logFormat)
		logger.SetFormatter(&logrus.JSONFormatter{TimestampFormat: tsFormat, FieldMap: fieldMap()})
	}

	logger.SetOutput(os.Stdout)
	return logger
}

// GetLogger returns a configured logrus logger instance
func GetLogger() *logrus.Logger {
	return SetupLogger()
}
