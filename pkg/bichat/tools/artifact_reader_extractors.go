package tools

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"path"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/extrame/xls"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/xuri/excelize/v2"
	"rsc.io/pdf"
)

func (t *ArtifactReaderTool) extractArtifactContent(ctx context.Context, artifact domain.Artifact, data []byte) (string, error) {
	ext := artifactExtension(artifact)
	mimeType := strings.ToLower(strings.TrimSpace(artifact.MimeType()))

	switch {
	case ext == ".csv" || mimeType == "text/csv":
		return extractDelimitedTable(data, ',')
	case ext == ".tsv" || mimeType == "text/tab-separated-values":
		return extractDelimitedTable(data, '\t')
	case ext == ".xlsx" || mimeType == "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return extractXLSX(data)
	case ext == ".xls" || mimeType == "application/vnd.ms-excel":
		return extractXLS(data)
	case ext == ".docx" || mimeType == "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return extractDOCX(data)
	case ext == ".pdf" || mimeType == "application/pdf":
		return t.extractPDF(ctx, artifact, data)
	case isTextLike(ext, mimeType):
		return decodeTextContent(data)
	default:
		return "", fmt.Errorf("unsupported artifact format: mime=%s ext=%s", artifact.MimeType(), ext)
	}
}

func artifactExtension(artifact domain.Artifact) string {
	if name := strings.TrimSpace(artifact.Name()); name != "" {
		if ext := strings.ToLower(path.Ext(name)); ext != "" {
			return ext
		}
	}
	if rawURL := strings.TrimSpace(artifact.URL()); rawURL != "" {
		if parsed, err := url.Parse(rawURL); err == nil {
			if ext := strings.ToLower(path.Ext(parsed.Path)); ext != "" {
				return ext
			}
		}
	}
	return ""
}

func isTextLike(ext string, mimeType string) bool {
	switch ext {
	case ".txt", ".md", ".json", ".xml", ".yaml", ".yml", ".log":
		return true
	}
	if strings.HasPrefix(mimeType, "text/") {
		return true
	}
	switch mimeType {
	case "application/json", "application/xml", "text/xml", "application/yaml", "text/yaml", "application/x-yaml", "text/x-yaml":
		return true
	default:
		return false
	}
}

func decodeTextContent(data []byte) (string, error) {
	if len(data) == 0 {
		return "", nil
	}

	if utf8.Valid(data) {
		return string(data), nil
	}

	decoded := string(data)
	totalRunes := 0
	invalidRunes := 0
	for _, r := range decoded {
		totalRunes++
		if r == utf8.RuneError {
			invalidRunes++
		}
	}

	if totalRunes == 0 {
		return "", nil
	}
	if invalidRunes*100/totalRunes > 10 {
		return "", fmt.Errorf("binary-like content unsupported")
	}

	return decoded, nil
}

func extractDelimitedTable(data []byte, delimiter rune) (string, error) {
	reader := csv.NewReader(bytes.NewReader(data))
	reader.Comma = delimiter
	reader.FieldsPerRecord = -1

	records, err := reader.ReadAll()
	if err != nil {
		return "", fmt.Errorf("failed to parse delimited content: %w", err)
	}
	if len(records) == 0 {
		return "No rows found.", nil
	}

	return rowsToMarkdown(records), nil
}

func extractXLSX(data []byte) (string, error) {
	book, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("failed to parse xlsx: %w", err)
	}
	defer func() { _ = book.Close() }()

	var lines []string
	for _, sheet := range book.GetSheetList() {
		rows, err := book.GetRows(sheet)
		if err != nil {
			continue
		}
		if len(rows) == 0 {
			continue
		}
		lines = append(lines, "### Sheet: "+sheet)
		lines = append(lines, rowsToMarkdownLines(rows)...)
		lines = append(lines, "")
	}

	if len(lines) == 0 {
		return "No worksheet rows found.", nil
	}

	return strings.TrimSpace(strings.Join(lines, "\n")), nil
}

func extractXLS(data []byte) (string, error) {
	book, err := xls.OpenReader(bytes.NewReader(data), "utf-8")
	if err != nil {
		return "", fmt.Errorf("failed to parse xls: %w", err)
	}

	var lines []string
	for sheetIdx := 0; sheetIdx < book.NumSheets(); sheetIdx++ {
		sheet := book.GetSheet(sheetIdx)
		if sheet == nil {
			continue
		}
		rows := make([][]string, 0)
		for rowIdx := 0; rowIdx <= int(sheet.MaxRow); rowIdx++ {
			row := sheet.Row(rowIdx)
			if row == nil {
				continue
			}
			lastCol := row.LastCol()
			if lastCol <= 0 {
				continue
			}
			cells := make([]string, 0, lastCol)
			for colIdx := 0; colIdx < lastCol; colIdx++ {
				cells = append(cells, strings.TrimSpace(row.Col(colIdx)))
			}
			rows = append(rows, cells)
		}

		if len(rows) == 0 {
			continue
		}

		title := sheet.Name
		if title == "" {
			title = "Sheet " + strconv.Itoa(sheetIdx+1)
		}
		lines = append(lines, "### Sheet: "+title)
		lines = append(lines, rowsToMarkdownLines(rows)...)
		lines = append(lines, "")
	}

	if len(lines) == 0 {
		return "No worksheet rows found.", nil
	}

	return strings.TrimSpace(strings.Join(lines, "\n")), nil
}

func extractDOCX(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("failed to open docx archive: %w", err)
	}

	var documentXML []byte
	for _, file := range zr.File {
		if file.Name != "word/document.xml" {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return "", fmt.Errorf("failed to open document.xml: %w", err)
		}
		documentXML, err = io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return "", fmt.Errorf("failed to read document.xml: %w", err)
		}
		break
	}

	if len(documentXML) == 0 {
		return "", fmt.Errorf("document.xml not found in docx")
	}

	decoder := xml.NewDecoder(bytes.NewReader(documentXML))
	var builder strings.Builder
	inText := false

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("failed to parse document.xml: %w", err)
		}

		switch elem := token.(type) {
		case xml.StartElement:
			switch elem.Name.Local {
			case "t":
				inText = true
			}
		case xml.EndElement:
			switch elem.Name.Local {
			case "t":
				inText = false
			case "p":
				builder.WriteString("\n")
			}
		case xml.CharData:
			if inText {
				builder.WriteString(string(elem))
			}
		}
	}

	return strings.TrimSpace(builder.String()), nil
}

func (t *ArtifactReaderTool) extractPDF(ctx context.Context, artifact domain.Artifact, data []byte) (string, error) {
	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("failed to parse pdf: %w", err)
	}

	var builder strings.Builder
	for pageIdx := 1; pageIdx <= reader.NumPage(); pageIdx++ {
		page := reader.Page(pageIdx)
		if page.V.IsNull() {
			continue
		}
		content := page.Content()
		for _, text := range content.Text {
			segment := strings.TrimSpace(text.S)
			if segment == "" {
				continue
			}
			builder.WriteString(segment)
			builder.WriteString(" ")
		}
		builder.WriteString("\n")
	}

	extracted := strings.TrimSpace(builder.String())
	if len(strings.Fields(extracted)) > 3 {
		return extracted, nil
	}

	if t.pdfFallback != nil {
		fallback, err := t.pdfFallback.ExtractPDFText(ctx, artifact.Name(), data)
		if err == nil && strings.TrimSpace(fallback) != "" {
			return fallback, nil
		}
	}

	return "This PDF has no extractable text and fallback is unavailable.", nil
}

func rowsToMarkdown(records [][]string) string {
	return strings.Join(rowsToMarkdownLines(records), "\n")
}

func rowsToMarkdownLines(records [][]string) []string {
	if len(records) == 0 {
		return []string{"No rows found."}
	}

	headers := make([]string, 0, len(records[0]))
	for idx, cell := range records[0] {
		value := strings.TrimSpace(cell)
		if value == "" {
			value = "column_" + strconv.Itoa(idx+1)
		}
		headers = append(headers, escapePipe(value))
	}
	if len(headers) == 0 {
		headers = []string{"column_1"}
	}

	lines := []string{
		"| " + strings.Join(headers, " | ") + " |",
		"| " + strings.Join(repeat("---", len(headers)), " | ") + " |",
	}

	for _, row := range records[1:] {
		values := make([]string, len(headers))
		for idx := range headers {
			if idx < len(row) {
				values[idx] = escapePipe(strings.TrimSpace(row[idx]))
			}
		}
		lines = append(lines, "| "+strings.Join(values, " | ")+" |")
	}

	return lines
}

func repeat(value string, count int) []string {
	out := make([]string, count)
	for i := 0; i < count; i++ {
		out[i] = value
	}
	return out
}

func escapePipe(value string) string {
	replacer := strings.NewReplacer("|", "\\|", "\n", " ", "\r", " ")
	return replacer.Replace(value)
}
