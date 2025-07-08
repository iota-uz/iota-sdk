package services

import (
	"bytes"
	"sync"
)

const (
	// MaxLogSize is the maximum size of log buffer per service (10MB)
	MaxLogSize = 10 * 1024 * 1024
	// LogTrimSize is how much to trim when buffer is full (1MB)
	LogTrimSize = 1024 * 1024
)

// CircularLogBuffer implements a size-limited log buffer
// When the buffer exceeds MaxLogSize, it removes old data
type CircularLogBuffer struct {
	mu       sync.RWMutex
	data     []byte
	maxSize  int
	trimSize int
}

func NewCircularLogBuffer() *CircularLogBuffer {
	return &CircularLogBuffer{
		data:     make([]byte, 0, MaxLogSize/10), // Start with 1/10th capacity
		maxSize:  MaxLogSize,
		trimSize: LogTrimSize,
	}
}

func (b *CircularLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Check if we need to trim
	if len(b.data)+len(p) > b.maxSize {
		// Find a good trim point (after a newline)
		trimPoint := b.trimSize
		for i := b.trimSize; i < len(b.data) && i < b.trimSize+1024; i++ {
			if b.data[i] == '\n' {
				trimPoint = i + 1
				break
			}
		}

		// Trim old data
		if trimPoint < len(b.data) {
			copy(b.data, b.data[trimPoint:])
			b.data = b.data[:len(b.data)-trimPoint]
		} else {
			// If trim point is beyond data, clear it
			b.data = b.data[:0]
		}
	}

	b.data = append(b.data, p...)
	return len(p), nil
}

// Bytes returns a copy of the buffer content
func (b *CircularLogBuffer) Bytes() []byte {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Return a copy to avoid race conditions
	result := make([]byte, len(b.data))
	copy(result, b.data)
	return result
}

// LastBytes returns the last n bytes from the buffer
// This is more efficient than getting all bytes when only recent logs are needed
func (b *CircularLogBuffer) LastBytes(n int) []byte {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if n >= len(b.data) {
		result := make([]byte, len(b.data))
		copy(result, b.data)
		return result
	}

	start := len(b.data) - n
	result := make([]byte, n)
	copy(result, b.data[start:])
	return result
}

// Reset clears the buffer
func (b *CircularLogBuffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.data = b.data[:0]
}

// Size returns the current size of the buffer
func (b *CircularLogBuffer) Size() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.data)
}

// ParseLastLines returns the last n lines from the buffer
// This is optimized for the log view which typically shows recent logs
func (b *CircularLogBuffer) ParseLastLines(n int) []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.data) == 0 {
		return nil
	}

	lines := make([]string, 0, n)
	lineStart := len(b.data)

	// Scan backwards to find line breaks
	for i := len(b.data) - 1; i >= 0 && len(lines) < n; i-- {
		if b.data[i] == '\n' || i == 0 {
			start := i
			if b.data[i] == '\n' {
				start = i + 1
			}
			if start < lineStart {
				line := string(b.data[start:lineStart])
				if line != "" {
					lines = append(lines, line)
				}
			}
			lineStart = i
		}
	}

	// Reverse to get chronological order
	for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
		lines[i], lines[j] = lines[j], lines[i]
	}

	return lines
}

// LineIterator provides an efficient way to iterate through log lines
// without parsing the entire buffer
type LineIterator struct {
	data []byte
	pos  int
}

func (b *CircularLogBuffer) NewLineIterator() *LineIterator {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Make a copy for safe iteration
	data := make([]byte, len(b.data))
	copy(data, b.data)

	return &LineIterator{
		data: data,
		pos:  0,
	}
}

func (li *LineIterator) Next() (string, bool) {
	if li.pos >= len(li.data) {
		return "", false
	}

	start := li.pos
	for li.pos < len(li.data) && li.data[li.pos] != '\n' {
		li.pos++
	}

	line := string(li.data[start:li.pos])
	if li.pos < len(li.data) {
		li.pos++ // Skip the newline
	}

	return line, true
}

// Search efficiently searches for a pattern in the buffer
func (b *CircularLogBuffer) Search(pattern []byte) []int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var matches []int
	offset := 0

	for {
		idx := bytes.Index(b.data[offset:], pattern)
		if idx == -1 {
			break
		}
		matches = append(matches, offset+idx)
		offset += idx + 1
	}

	return matches
}
