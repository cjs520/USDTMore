package log

import (
	"github.com/sirupsen/logrus"
	"io"
	"os"
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

	// 输出到标准输出而不是文件
	logger.SetOutput(os.Stdout)
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
