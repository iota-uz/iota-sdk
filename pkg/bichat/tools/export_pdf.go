package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// ExportToPDFTool exports content to PDF format using Gotenberg.
// Gotenberg is a Docker-powered stateless API for converting HTML, Markdown, and Office documents to PDF.
// See: https://gotenberg.dev/
type ExportToPDFTool struct {
	gotenbergURL string
}

// NewExportToPDFTool creates a new export to PDF tool.
// gotenbergURL is the base URL of the Gotenberg service (e.g., "http://localhost:3000").
func NewExportToPDFTool(gotenbergURL string) agents.Tool {
	return &ExportToPDFTool{
		gotenbergURL: gotenbergURL,
	}
}

// Name returns the tool name.
func (t *ExportToPDFTool) Name() string {
	return "export_to_pdf"
}

// Description returns the tool description for the LLM.
func (t *ExportToPDFTool) Description() string {
	return "Export content to PDF format. " +
		"Accepts HTML content and converts it to a PDF file. " +
		"Returns a download URL for the generated PDF."
}

// Parameters returns the JSON Schema for tool parameters.
func (t *ExportToPDFTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"html": map[string]any{
				"type":        "string",
				"description": "HTML content to convert to PDF",
			},
			"filename": map[string]any{
				"type":        "string",
				"description": "Filename for the PDF file (default: 'export.pdf')",
				"default":     "export.pdf",
			},
			"landscape": map[string]any{
				"type":        "boolean",
				"description": "Whether to use landscape orientation (default: false)",
				"default":     false,
			},
		},
		"required": []string{"html"},
	}
}

// pdfExportInput represents the parsed input parameters.
type pdfExportInput struct {
	HTML      string `json:"html"`
	Filename  string `json:"filename,omitempty"`
	Landscape bool   `json:"landscape,omitempty"`
}

// pdfExportOutput represents the formatted output.
type pdfExportOutput struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
	Size     int64  `json:"size,omitempty"`
}

// Call executes the PDF export operation.
func (t *ExportToPDFTool) Call(ctx context.Context, input string) (string, error) {
	const op serrors.Op = "ExportToPDFTool.Call"

	// Parse input
	params, err := agents.ParseToolInput[pdfExportInput](input)
	if err != nil {
		return "", serrors.E(op, err, "failed to parse input")
	}

	if params.HTML == "" {
		return "", serrors.E(op, "html parameter is required")
	}

	// Set defaults
	filename := params.Filename
	if filename == "" {
		filename = "export.pdf"
	}

	// Convert HTML to PDF using Gotenberg
	pdfData, err := t.convertHTMLToPDF(ctx, params.HTML, params.Landscape)
	if err != nil {
		return "", serrors.E(op, err, "PDF conversion failed")
	}

	// TODO: Save PDF file and return URL
	// This is a placeholder - consumers should implement file storage
	url := fmt.Sprintf("/exports/%s", filename)

	// Build response
	response := pdfExportOutput{
		URL:      url,
		Filename: filename,
		Size:     int64(len(pdfData)),
	}

	return agents.FormatToolOutput(response)
}

// convertHTMLToPDF converts HTML content to PDF using Gotenberg.
func (t *ExportToPDFTool) convertHTMLToPDF(ctx context.Context, html string, landscape bool) ([]byte, error) {
	const op serrors.Op = "ExportToPDFTool.convertHTMLToPDF"

	// Prepare request body
	requestBody := map[string]interface{}{
		"html": html,
		"properties": map[string]interface{}{
			"landscape": landscape,
		},
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, serrors.E(op, err, "failed to marshal request")
	}

	// Make request to Gotenberg
	url := fmt.Sprintf("%s/forms/chromium/convert/html", strings.TrimRight(t.gotenbergURL, "/"))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, serrors.E(op, err, "failed to create request")
	}

	req.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, serrors.E(op, err, "failed to send request")
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// TODO: log response body close error
			_ = err
		}
	}()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, serrors.E(op, fmt.Sprintf("Gotenberg returned status %d: %s", resp.StatusCode, string(body)))
	}

	// Read PDF data
	pdfData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, serrors.E(op, err, "failed to read PDF data")
	}

	return pdfData, nil
}

// PDFExporter defines the interface for exporting content to PDF.
// Consumers can implement this interface to customize PDF generation.
type PDFExporter interface {
	// ExportToPDF exports HTML content to a PDF file and returns the file path or URL.
	ExportToPDF(ctx context.Context, html string, filename string, landscape bool) (string, error)
}

// GotenbergPDFExporter implements PDFExporter using Gotenberg.
type GotenbergPDFExporter struct {
	gotenbergURL string
	outputDir    string
	baseURL      string
}

// NewGotenbergPDFExporter creates a new Gotenberg PDF exporter.
// gotenbergURL is the base URL of the Gotenberg service.
// outputDir is the directory where PDF files will be saved.
// baseURL is the base URL for download links.
func NewGotenbergPDFExporter(gotenbergURL, outputDir, baseURL string) PDFExporter {
	return &GotenbergPDFExporter{
		gotenbergURL: gotenbergURL,
		outputDir:    outputDir,
		baseURL:      baseURL,
	}
}

// ExportToPDF exports HTML content to a PDF file.
func (e *GotenbergPDFExporter) ExportToPDF(ctx context.Context, html string, filename string, landscape bool) (string, error) {
	const op serrors.Op = "GotenbergPDFExporter.ExportToPDF"

	tool := NewExportToPDFTool(e.gotenbergURL)
	pdfTool := tool.(*ExportToPDFTool)

	// Convert HTML to PDF
	pdfData, err := pdfTool.convertHTMLToPDF(ctx, html, landscape)
	if err != nil {
		return "", serrors.E(op, err)
	}

	// TODO: Save PDF file to outputDir
	// This is a placeholder - implement file storage
	_ = pdfData

	// Return download URL
	url := fmt.Sprintf("%s/%s", e.baseURL, filename)
	return url, nil
}
