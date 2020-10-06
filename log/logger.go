package log

import (
	"io"
	"log"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/swisscom/backman/config"
	"go.elastic.co/apm/module/apmlogrus"
)

var (
	logger *logrus.Logger
)

func init() {
	logger = newLogger(os.Stdout)
}

func newLogger(writer io.Writer) *logrus.Logger {
	logLevel, err := logrus.ParseLevel(config.Get().LogLevel)
	if err != nil {
		log.Fatal(err)
	}

	logger := logrus.New()
	logger.SetOutput(writer)
	logger.SetLevel(logLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		QuoteEmptyFields: true,
		DisableColors:    true,
		FullTimestamp:    false,
		DisableTimestamp: !config.Get().LoggingTimestamp,
	})

	if config.Get().EnableAPM == true {
		logrus.AddHook(&apmlogrus.Hook{})
	}

	return logger
}

func Printf(format string, args ...interface{}) {
	logger.Infof(format, args...)
}

func Println(args ...interface{}) {
	logger.Infoln(args...)
}

func Infof(format string, args ...interface{}) {
	logger.Infof(format, args...)
}

func Infoln(args ...interface{}) {
	logger.Infoln(args...)
}

func Warnf(format string, args ...interface{}) {
	logger.Warnf(format, args...)
}

func Warnln(args ...interface{}) {
	logger.Warnln(args...)
}

func Debugf(format string, args ...interface{}) {
	logger.Debugf(format, args...)
}

func Debugln(args ...interface{}) {
	logger.Debugln(args...)
}

func Errorf(format string, args ...interface{}) {
	logger.Errorf(format, args...)
}

func Errorln(args ...interface{}) {
	logger.Errorln(args...)
}

func Fatalf(format string, args ...interface{}) {
	logger.Fatalf(format, args...)
}

func Fatalln(args ...interface{}) {
	logger.Fatalln(args...)
}
