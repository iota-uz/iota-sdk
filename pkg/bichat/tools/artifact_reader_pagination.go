package tools

import "strings"

const (
	artifactReaderMinPageSize   = 10
	artifactReaderMaxPageSize   = 400
	artifactReaderDefaultPage   = 1
	artifactReaderDefaultSize   = 100
	artifactReaderMaxOutputSize = 32000
)

type pageWindow struct {
	Page       int
	PageSize   int
	TotalPages int
	HasNext    bool
	HasPrev    bool
	OutOfRange bool
	Lines      []string
}

func clampPage(page int) int {
	if page < 1 {
		return artifactReaderDefaultPage
	}
	return page
}

func clampPageSize(pageSize int) int {
	if pageSize < artifactReaderMinPageSize {
		return artifactReaderMinPageSize
	}
	if pageSize > artifactReaderMaxPageSize {
		return artifactReaderMaxPageSize
	}
	return pageSize
}

func paginateLines(lines []string, page int, pageSize int) pageWindow {
	page = clampPage(page)
	pageSize = clampPageSize(pageSize)

	totalItems := len(lines)
	totalPages := 1
	if totalItems > 0 {
		totalPages = (totalItems + pageSize - 1) / pageSize
	}

	if page > totalPages {
		return pageWindow{
			Page:       page,
			PageSize:   pageSize,
			TotalPages: totalPages,
			HasNext:    false,
			HasPrev:    totalPages > 0,
			OutOfRange: true,
			Lines:      nil,
		}
	}

	start := (page - 1) * pageSize
	if start < 0 {
		start = 0
	}
	if start > totalItems {
		start = totalItems
	}
	end := start + pageSize
	if end > totalItems {
		end = totalItems
	}

	pageLines := make([]string, 0, end-start)
	if start < end {
		pageLines = append(pageLines, lines[start:end]...)
	}

	return pageWindow{
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
		HasNext:    page < totalPages,
		HasPrev:    page > 1,
		OutOfRange: false,
		Lines:      pageLines,
	}
}

func normalizeToLines(content string) []string {
	if content == "" {
		return nil
	}
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	return strings.Split(normalized, "\n")
}
