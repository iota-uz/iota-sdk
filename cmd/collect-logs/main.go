package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
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

func main() {
	config := configuration.Use()
	
	if config.Loki.URL == "" {
		log.Fatal("Loki URL is not configured")
	}
	
	if config.Loki.AppName == "" {
		log.Fatal("Loki app name is not configured")
	}
	
	logPath := config.Loki.LogPath
	if logPath == "" {
		log.Fatal("Log path is not configured")
	}
	
	log.Printf("Starting continuous log collector, watching %s", logPath)
	
	processLogFile(config.Loki.URL, config.Loki.AppName, logPath)
}

func processLogFile(lokiURL, appName, logPath string) {
	for {
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			log.Printf("Log file does not exist: %s. Waiting for it to be created...", logPath)
			time.Sleep(5 * time.Second)
			continue
		}

		client := &http.Client{
			Timeout: 5 * time.Second,
		}

		file, err := os.Open(logPath)
		if err != nil {
			log.Printf("Failed to open log file: %v. Retrying in 5 seconds...", err)
			time.Sleep(5 * time.Second)
			continue
		}
		
		fileInfo, err := file.Stat()
		if err != nil {
			log.Printf("Failed to get file info: %v. Retrying in 5 seconds...", err)
			file.Close()
			time.Sleep(5 * time.Second)
			continue
		}
		
		offset := fileInfo.Size()
		if _, err := file.Seek(offset, 0); err != nil {
			log.Printf("Failed to seek to end of file: %v. Retrying in 5 seconds...", err)
			file.Close()
			time.Sleep(5 * time.Second)
			continue
		}

		scanner := bufio.NewScanner(file)
		
		var batch []map[string]interface{}
		batchSize := 100
		batchTimeout := 5 * time.Second
		lastBatchTime := time.Now()

		var lineCount int
		
		processBatch := func() {
			if len(batch) > 0 {
				if err := sendBatchToLoki(client, lokiURL, appName, batch); err != nil {
					log.Printf("Failed to send batch to Loki: %v", err)
				} else {
					log.Printf("Sent %d log entries to Loki", len(batch))
				}
				
				batch = nil
				lastBatchTime = time.Now()
			}
		}
		
		for {
			hasMore := scanner.Scan()
			if !hasMore {
				if err := scanner.Err(); err != nil {
					log.Printf("Error reading log file: %v. Reopening in 5 seconds...", err)
					break
				}
				
				if time.Since(lastBatchTime) >= batchTimeout {
					processBatch()
				}
				
				time.Sleep(1 * time.Second)
				
				newFileInfo, err := os.Stat(logPath)
				if err != nil {
					if os.IsNotExist(err) {
						log.Printf("Log file has been removed, waiting for it to be recreated")
						break
					}
					log.Printf("Failed to get file info: %v", err)
					break
				}
				
				if newFileInfo.Size() < offset {
					log.Printf("Log file appears to have been rotated, reopening")
					break
				}
				
				fileInfo, err = file.Stat()
				if err != nil {
					log.Printf("Failed to get file info: %v", err)
					break
				}
				
				if fileInfo.Size() <= offset {
					continue
				}
				
				if _, err := file.Seek(offset, 0); err != nil {
					log.Printf("Failed to seek in file: %v", err)
					break
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
			
			if len(batch) >= batchSize || time.Since(lastBatchTime) >= batchTimeout {
				processBatch()
			}
		}
		
		processBatch()
		
		file.Close()
		
		log.Printf("Reopening log file in 5 seconds...")
		time.Sleep(5 * time.Second)
	}
}

func sendBatchToLoki(client *http.Client, lokiURL, appName string, batch []map[string]interface{}) error {
	stream := LokiStream{
		Stream: map[string]string{
			"app": appName,
		},
		Values: make([][2]string, 0, len(batch)),
	}
	
	for _, logEntry := range batch {
		var timestamp int64
		if ts, ok := logEntry["timestamp"].(float64); ok {
			timestamp = int64(ts)
		} else if ts, ok := logEntry["time"].(string); ok {
			t, err := time.Parse(time.RFC3339, ts)
			if err != nil {
				log.Printf("Failed to parse timestamp %s: %v", ts, err)
				timestamp = time.Now().UnixNano()
			} else {
				timestamp = t.UnixNano()
			}
		} else {
			timestamp = time.Now().UnixNano()
		}
		
		if level, ok := logEntry["level"].(string); ok {
			stream.Stream["level"] = level
		}
		
		if requestID, ok := logEntry["request-id"].(string); ok {
			stream.Stream["request_id"] = requestID
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
	
	push := LokiPush{
		Streams: []LokiStream{stream},
	}
	
	buf, err := json.Marshal(push)
	if err != nil {
		return fmt.Errorf("failed to marshal Loki payload: %w", err)
	}
	
	req, err := http.NewRequest("POST", lokiURL, bytes.NewBuffer(buf))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send data to Loki: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("Loki returned status code %d, but failed to read response body: %w", resp.StatusCode, err)
		}
		
		return fmt.Errorf("Loki returned status code %d: %s", resp.StatusCode, string(respBody))
	}
	
	return nil
}