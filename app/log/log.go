package log

import (
	"USDTMore/app/config"
	"io"
	"os"

	"github.com/sirupsen/logrus"
)

var logger *logrus.Logger

func init() {
	var level = logrus.InfoLevel
	logger = logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		ForceColors:     true,
		ForceQuote:      true,
		TimestampFormat: "2006-01-02 15:04:05",
		FullTimestamp:   true,
	})

	logger.SetLevel(level)

	// 检查是否在Docker环境中运行，如果是则输出到stdout，否则输出到文件
	if os.Getenv("DOCKER_ENV") == "true" || os.Getenv("LOG_TO_STDOUT") == "true" {
		// Docker环境或明确指定输出到stdout
		logger.SetOutput(os.Stdout)
	} else {
		// 传统文件输出
		logFile := config.GetOutputLog()
		output, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			// 如果文件创建失败，回退到stdout
			logger.SetOutput(os.Stdout)
			logger.Warnf("无法创建日志文件 %s，回退到标准输出: %v", logFile, err)
		} else {
			logger.SetOutput(output)
		}
	}
}

func Debug(args ...interface{}) {
	logger.Debugln(args...)
}

func Info(args ...interface{}) {
	logger.Infoln(args...)
}

func Error(args ...interface{}) {
	logger.Errorln(args...)
}

func Warn(args ...interface{}) {
	logger.Warnln(args...)
}

func GetWriter() *io.PipeWriter {
	return logger.Writer()
}
