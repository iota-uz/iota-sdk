package devhub

import (
	"strings"
	"sync"
)

// LogCache caches parsed log lines to avoid re-parsing on every render
type LogCache struct {
	mu         sync.RWMutex
	rawData    []byte
	lines      []string
	lastSize   int
	serviceIdx int
}

func NewLogCache() *LogCache {
	return &LogCache{
		lines: make([]string, 0),
	}
}

// GetLines returns cached lines if the data hasn't changed,
// otherwise it re-parses the logs
func (lc *LogCache) GetLines(serviceIdx int, rawLogs []byte) []string {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	// Check if we can use cached data
	if serviceIdx == lc.serviceIdx &&
		len(rawLogs) == lc.lastSize &&
		len(lc.lines) > 0 {
		return lc.lines
	}

	// Parse logs
	lc.serviceIdx = serviceIdx
	lc.lastSize = len(rawLogs)
	lc.rawData = rawLogs

	if len(rawLogs) == 0 {
		lc.lines = []string{}
		return lc.lines
	}

	// Efficient line splitting
	lc.lines = strings.Split(string(rawLogs), "\n")

	return lc.lines
}

// GetVisibleLines returns only the lines that should be rendered
// based on the scroll position and viewport height
func (lc *LogCache) GetVisibleLines(serviceIdx int, rawLogs []byte, scrollPos int, viewportHeight int) ([]string, int) {
	lines := lc.GetLines(serviceIdx, rawLogs)

	if len(lines) == 0 {
		return []string{}, 0
	}

	// Calculate visible range
	maxScroll := len(lines) - viewportHeight
	if maxScroll < 0 {
		maxScroll = 0
	}

	if scrollPos > maxScroll {
		scrollPos = maxScroll
	}
	if scrollPos < 0 {
		scrollPos = 0
	}

	endPos := scrollPos + viewportHeight
	if endPos > len(lines) {
		endPos = len(lines)
	}

	return lines[scrollPos:endPos], scrollPos
}

// SearchInLines performs efficient search within cached lines
func (lc *LogCache) SearchInLines(query string, caseSensitive bool) []int {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	if query == "" || len(lc.lines) == 0 {
		return []int{}
	}

	matches := make([]int, 0)
	searchQuery := query

	if !caseSensitive {
		searchQuery = strings.ToLower(query)
	}

	for i, line := range lc.lines {
		searchLine := line
		if !caseSensitive {
			searchLine = strings.ToLower(line)
		}

		if strings.Contains(searchLine, searchQuery) {
			matches = append(matches, i)
		}
	}

	return matches
}
