// pkg/log/logger.go
package log

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

var logger *logrus.Logger

func init() {
	logger = logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	logger.SetOutput(os.Stdout)
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006/01/02 - 15:04:05",
		ForceColors:     true,
	})

	logger.Debug("Logger initialized with TextFormatter.")
}

// Log returns the global logger instance.
func Log() *logrus.Logger {
	return logger
}

const (
	bgGreen    = "\x1b[42m"
	bgRed      = "\x1b[41m"
	bgYellow   = "\x1b[43m"
	bgBlue     = "\x1b[44m"
	bgMagenta  = "\x1b[45m"
	bgCyan     = "\x1b[46m"
	colorReset = "\x1b[0m"
)

var skippedGinLogPaths = map[string]struct{}{
	"/api/v1/monitor/agent/heartbeat": {},
	"/api/v1/cmdb/agent/heartbeat":    {},
	"/api/v1/cmdb/hosts":              {},
}

var skippedGinLogPathPrefixes = []string{
	"/api/v1/logtail/agent/",
}

func shouldSkipGinLogging(path string) bool {
	if _, ok := skippedGinLogPaths[path]; ok {
		return true
	}
	for skippedPath := range skippedGinLogPaths {
		if len(path) > len(skippedPath) && path[len(path)-len(skippedPath):] == skippedPath {
			return true
		}
	}
	for _, skippedPrefix := range skippedGinLogPathPrefixes {
		if strings.HasPrefix(path, skippedPrefix) || strings.Contains(path, skippedPrefix) {
			return true
		}
	}
	return false
}

func getBackgroundColorForStatusCode(code int) string {
	switch {
	case code >= 200 && code < 300:
		return bgGreen
	case code >= 400 && code < 500:
		return bgRed
	case code >= 500:
		return bgYellow
	default:
		return colorReset
	}
}

func getBackgroundColorForMethod(method string) string {
	switch method {
	case "GET":
		return bgBlue
	case "POST":
		return bgMagenta
	case "PUT":
		return bgCyan
	case "DELETE":
		return bgRed
	default:
		return colorReset
	}
}

func CustomGinLogger() gin.HandlerFunc {
	isTerminal := false
	if fileInfo, _ := os.Stdout.Stat(); fileInfo != nil {
		isTerminal = (fileInfo.Mode() & os.ModeCharDevice) != 0
	}

	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method
		ip := c.ClientIP()
		skipLogging := shouldSkipGinLogging(path)

		c.Next()

		if skipLogging {
			return
		}

		latency := time.Since(start)
		statusCode := c.Writer.Status()

		var logMessage string
		if isTerminal {
			statusBgColor := getBackgroundColorForStatusCode(statusCode)
			methodBgColor := getBackgroundColorForMethod(method)
			logMessage = fmt.Sprintf("[GIN] %s | %s%3d%s | %13v | %15s | %s%-6s%s %q",
				time.Now().Format("2006/01/02 - 15:04:05"),
				statusBgColor, statusCode, colorReset,
				latency,
				ip,
				methodBgColor, method, colorReset,
				path,
			)
		} else {
			logMessage = fmt.Sprintf("[GIN] %s | %3d | %13v | %15s | %-6s %q",
				time.Now().Format("2006/01/02 - 15:04:05"),
				statusCode,
				latency,
				ip,
				method,
				path,
			)
		}

		if statusCode >= 500 {
			logger.Error(logMessage)
		} else if statusCode >= 400 {
			logger.Warn(logMessage)
		} else {
			logger.Info(logMessage)
		}
	}
}

// Setup keeps compatibility with existing call sites.
func Setup() {
	if logger == nil {
		panic("Logger initialization failed")
	}
}
