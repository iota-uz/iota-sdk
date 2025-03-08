package logging

import (
	"io"
	"os"

	"github.com/sirupsen/logrus"
)

func FileLogger(level logrus.Level) (*os.File, *logrus.Logger, error) {
	logger := logrus.New()
	if _, err := os.Stat("./logs"); os.IsNotExist(err) {
		if err := os.Mkdir("./logs", os.ModePerm); err != nil {
			return nil, nil, err
		}
	}
	logFile, err := os.OpenFile("./logs/app.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, nil, err
	}

	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetOutput(io.MultiWriter(os.Stdout, logFile))
	logger.SetLevel(level)
	return logFile, logger, nil
}

func ConsoleLogger(level logrus.Level) *logrus.Logger {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetOutput(os.Stdout)
	logger.SetLevel(level)
	return logger
}
