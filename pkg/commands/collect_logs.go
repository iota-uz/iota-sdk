package commands

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/configuration"
)

type LokiStream struct {
	Stream map[string]string `json:"stream"`
	Values [][2]string       `json:"values"`
}

type LokiPush struct {
	Streams []LokiStream `json:"streams"`
}

type LogCollector struct {
	LokiURL   string
	AppName   string
	LogPath   string
	BatchSize int
	Timeout   time.Duration
	Labels    []string
}

// CollectLogs initializes and runs a log collector that forwards logs to Loki
func CollectLogs(ctx context.Context, options ...func(*LogCollector)) error {
	config := configuration.Use()

	if config.Loki.URL == "" {
		return fmt.Errorf("loki URL is not configured")
	}

	if config.Loki.AppName == "" {
		return fmt.Errorf("loki app name is not configured")
	}

	logPath := config.Loki.LogPath
	if logPath == "" {
		return fmt.Errorf("log path is not configured")
	}

	defaultLabels := []string{
		"level",
		"request-id",
		"path",
		"method",
		"host",
		"ip",
		"completed",
		"user-agent",
		"trace-id",
		"span-id",
	}

	collector := &LogCollector{
		LokiURL:   config.Loki.URL,
		AppName:   config.Loki.AppName,
		LogPath:   logPath,
		BatchSize: 100,
		Timeout:   5 * time.Second,
		Labels:    defaultLabels,
	}

	// Apply any custom options
	for _, option := range options {
		option(collector)
	}

	log.Printf("Starting continuous log collector, watching %s", collector.LogPath)

	// Use the provided context for cancellation
	go func() {
		<-ctx.Done()
		log.Println("Log collector stopping due to context cancellation")
	}()

	return collector.Process(ctx)
}

// WithLabels allows customizing the labels to extract from log entries
func WithLabels(labels []string) func(*LogCollector) {
	return func(c *LogCollector) {
		c.Labels = labels
	}
}

// WithBatchSize allows customizing the batch size for sending logs
func WithBatchSize(batchSize int) func(*LogCollector) {
	return func(c *LogCollector) {
		c.BatchSize = batchSize
	}
}

// WithTimeout allows customizing the timeout for HTTP requests
func WithTimeout(timeout time.Duration) func(*LogCollector) {
	return func(c *LogCollector) {
		c.Timeout = timeout
	}
}

// WithLogPath allows customizing the log file path
func WithLogPath(logPath string) func(*LogCollector) {
	return func(c *LogCollector) {
		c.LogPath = logPath
	}
}

// Process continuously monitors the log file and sends batches to Loki
func (c *LogCollector) Process(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Continue processing
		}

		if _, err := os.Stat(c.LogPath); os.IsNotExist(err) {
			log.Printf("Log file does not exist: %s. Waiting for it to be created...", c.LogPath)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
				continue
			}
		}

		client := &http.Client{
			Timeout: c.Timeout,
		}

		file, err := os.Open(c.LogPath)
		if err != nil {
			log.Printf("Failed to open log file: %v. Retrying in 5 seconds...", err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
				continue
			}
		}

		fileInfo, err := file.Stat()
		if err != nil {
			log.Printf("Failed to get file info: %v. Retrying in 5 seconds...", err)
			if err := file.Close(); err != nil {
				log.Printf("Failed to close file: %v", err)
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
				continue
			}
		}

		offset := fileInfo.Size()
		if _, err := file.Seek(offset, 0); err != nil {
			log.Printf("Failed to seek to end of file: %v. Retrying in 5 seconds...", err)
			if err := file.Close(); err != nil {
				log.Printf("Failed to close file: %v", err)
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
				continue
			}
		}

		scanner := bufio.NewScanner(file)

		var batch []map[string]interface{}
		batchTimeout := c.Timeout
		lastBatchTime := time.Now()

		var lineCount int

		processBatch := func() {
			if len(batch) > 0 {
				if err := c.SendBatch(ctx, client, batch); err != nil {
					log.Printf("Failed to send batch to Loki: %v", err)
				} else {
					log.Printf("Sent %d log entries to Loki", len(batch))
				}

				batch = nil
				lastBatchTime = time.Now()
			}
		}

	scannerLoop:
		for {
			select {
			case <-ctx.Done():
				processBatch() // Process any remaining logs before exiting
				_ = file.Close()
				return ctx.Err()
			default:
				// Continue processing
			}

			hasMore := scanner.Scan()
			if !hasMore {
				if err := scanner.Err(); err != nil {
					log.Printf("Error reading log file: %v. Reopening in 5 seconds...", err)
					break scannerLoop
				}

				if time.Since(lastBatchTime) >= batchTimeout {
					processBatch()
				}

				select {
				case <-ctx.Done():
					processBatch()
					_ = file.Close()
					return ctx.Err()
				case <-time.After(1 * time.Second):
					// Continue
				}

				newFileInfo, err := os.Stat(c.LogPath)
				if err != nil {
					if os.IsNotExist(err) {
						log.Printf("Log file has been removed, waiting for it to be recreated")
						break scannerLoop
					}
					log.Printf("Failed to get file info: %v", err)
					break scannerLoop
				}

				if newFileInfo.Size() < offset {
					log.Printf("Log file appears to have been rotated, reopening")
					break scannerLoop
				}

				fileInfo, err = file.Stat()
				if err != nil {
					log.Printf("Failed to get file info: %v", err)
					break scannerLoop
				}

				if fileInfo.Size() <= offset {
					continue
				}

				if _, err := file.Seek(offset, 0); err != nil {
					log.Printf("Failed to seek in file: %v", err)
					break scannerLoop
				}

				scanner = bufio.NewScanner(file)

				continue
			}

			line := scanner.Text()
			lineCount++
			offset += int64(len(line)) + 1

			if strings.TrimSpace(line) == "" {
				continue
			}

			var logEntry map[string]interface{}
			if err := json.Unmarshal([]byte(line), &logEntry); err != nil {
				log.Printf("Failed to parse log line %d: %v", lineCount, err)
				continue
			}

			batch = append(batch, logEntry)

			if len(batch) >= c.BatchSize || time.Since(lastBatchTime) >= batchTimeout {
				processBatch()
			}
		}

		processBatch()

		if err := file.Close(); err != nil {
			log.Printf("Failed to close file: %v", err)
		}

		log.Printf("Reopening log file in 5 seconds...")

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
			continue
		}
	}
}

// SendBatch sends a batch of log entries to Loki
func (c *LogCollector) SendBatch(ctx context.Context, client *http.Client, batch []map[string]interface{}) error {
	streamsByLabels := make(map[string]*LokiStream)

	for _, logEntry := range batch {
		labels := map[string]string{
			"app": c.AppName,
		}

		for _, label := range c.Labels {
			labelKey := strings.ReplaceAll(label, "-", "_")

			if value, ok := logEntry[label]; ok {
				switch v := value.(type) {
				case string:
					labels[labelKey] = v
				case bool:
					labels[labelKey] = strconv.FormatBool(v)
				case float64:
					labels[labelKey] = strconv.FormatFloat(v, 'f', -1, 64)
				case int:
					labels[labelKey] = strconv.Itoa(v)
				case int64:
					labels[labelKey] = strconv.FormatInt(v, 10)
				case map[string]interface{}, []interface{}:
					continue
				default:
					labels[labelKey] = fmt.Sprintf("%v", v)
				}
			}
		}

		labelKey := createLabelKey(labels)

		stream, ok := streamsByLabels[labelKey]
		if !ok {
			stream = &LokiStream{
				Stream: labels,
				Values: make([][2]string, 0),
			}
			streamsByLabels[labelKey] = stream
		}

		var timestamp int64
		if ts, ok := logEntry["timestamp"].(float64); ok {
			timestamp = int64(ts)
		} else {
			timestamp = time.Now().UnixNano()
		}

		jsonData, err := json.Marshal(logEntry)
		if err != nil {
			log.Printf("Failed to marshal log entry: %v", err)
			continue
		}

		stream.Values = append(stream.Values, [2]string{
			strconv.FormatInt(timestamp, 10),
			string(jsonData),
		})
	}

	streams := make([]LokiStream, 0, len(streamsByLabels))
	for _, stream := range streamsByLabels {
		streams = append(streams, *stream)
	}

	push := LokiPush{
		Streams: streams,
	}

	buf, err := json.Marshal(push)
	if err != nil {
		return fmt.Errorf("failed to marshal Loki payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.LokiURL, bytes.NewBuffer(buf))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send data to Loki: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("Loki returned status code %d, but failed to read response body: %w", resp.StatusCode, err)
		}

		return fmt.Errorf("Loki returned status code %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// createLabelKey creates a unique key for a set of labels
func createLabelKey(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(labels[k])
	}

	return b.String()
}
