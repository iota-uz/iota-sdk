package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// Config represents the configuration for LokiHook
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
	// Skip sending to Loki if URL is empty
	if h.URL == "" {
		return nil
	}

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