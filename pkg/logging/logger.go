package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
)

func FileLogger(level logrus.Level, logPath string) (*os.File, *logrus.Logger, error) {
	logger := logrus.New()
	
	// Create logs directory if it doesn't exist
	logDir := filepath.Dir(logPath)
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		if err := os.MkdirAll(logDir, os.ModePerm); err != nil {
			return nil, nil, err
		}
	}
	
	logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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

// LokiConfig represents the configuration for LokiHook
type LokiConfig struct {
	Labels map[string]string
	Client *http.Client
}

// LokiHook sends logs to Loki
type LokiHook struct {
	URL     string
	AppName string
	Config  *LokiConfig
	client  *http.Client
}

// LokiStream represents a stream of log entries in Loki format
type LokiStream struct {
	Stream map[string]string `json:"stream"`
	Values [][2]string       `json:"values"`
}

// LokiPush represents the payload sent to Loki's push API
type LokiPush struct {
	Streams []LokiStream `json:"streams"`
}

// NewLokiHook creates a new Loki hook
func NewLokiHook(url, appName string, cfg *LokiConfig) (*LokiHook, error) {
	if url == "" {
		return nil, fmt.Errorf("Loki URL is required")
	}

	var client *http.Client
	if cfg != nil && cfg.Client != nil {
		client = cfg.Client
	} else {
		client = &http.Client{
			Timeout: 5 * time.Second,
		}
	}

	return &LokiHook{
		URL:     url,
		AppName: appName,
		Config:  cfg,
		client:  client,
	}, nil
}

// Fire is called when a log event is fired
func (h *LokiHook) Fire(entry *logrus.Entry) error {
	stream := map[string]string{
		"app": h.AppName,
	}

	// Add custom labels if provided
	if h.Config != nil && h.Config.Labels != nil {
		for k, v := range h.Config.Labels {
			stream[k] = v
		}
	}

	// Add level as a label
	stream["level"] = entry.Level.String()

	// Format the log entry as JSON
	data, err := entry.WithField("timestamp", entry.Time.UnixNano()).String()
	if err != nil {
		return err
	}

	// Create the Loki payload
	push := LokiPush{
		Streams: []LokiStream{
			{
				Stream: stream,
				Values: [][2]string{
					{
						fmt.Sprintf("%d", time.Now().UnixNano()),
						data,
					},
				},
			},
		},
	}

	// Marshal the payload to JSON
	buf, err := json.Marshal(push)
	if err != nil {
		return err
	}

	// Send the data to Loki
	req, err := http.NewRequest("POST", h.URL, bytes.NewBuffer(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// Levels returns the available logging levels
func (h *LokiHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// AddLokiHook adds a Loki hook to the logger
func AddLokiHook(logger *logrus.Logger, url, appName string) error {
	hook, err := NewLokiHook(url, appName, nil)
	if err != nil {
		return err
	}
	
	logger.AddHook(hook)
	return nil
}
