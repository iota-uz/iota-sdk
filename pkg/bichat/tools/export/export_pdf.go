package export

import (
	tools "github.com/iota-uz/iota-sdk/pkg/bichat/tools"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/logging"
	"github.com/iota-uz/iota-sdk/pkg/bichat/storage"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// ExportToPDFTool exports content to PDF format using Gotenberg.
// Gotenberg is a Docker-powered stateless API for converting HTML, Markdown, and Office documents to PDF.
// See: https://gotenberg.dev/
type ExportToPDFTool struct {
	gotenbergURL string
	storage      storage.FileStorage
	logger       logging.Logger
}

// PDFToolOption configures an ExportToPDFTool.
type PDFToolOption func(*ExportToPDFTool)

// WithLogger sets a logger for the PDF export tool.
func WithLogger(logger logging.Logger) PDFToolOption {
	return func(t *ExportToPDFTool) {
		t.logger = logger
	}
}

// NewExportToPDFTool creates a new export to PDF tool.
//
// Parameters:
//   - gotenbergURL: Base URL of the Gotenberg service (e.g., "http://localhost:3000")
//   - storage: File storage backend for saving generated PDFs
//   - opts: Optional configuration (logger)
//
// Example:
//
//	storage, _ := storage.NewLocalFileStorage("/var/lib/bichat/exports", "https://example.com/exports")
//	logger := logging.NewStdLogger()
//	tool := tools.NewExportToPDFTool("http://gotenberg:3000", storage, WithLogger(logger))
func NewExportToPDFTool(gotenbergURL string, fileStorage storage.FileStorage, opts ...PDFToolOption) agents.Tool {
	t := &ExportToPDFTool{
		gotenbergURL: gotenbergURL,
		storage:      fileStorage,
		logger:       logging.NewNoOpLogger(), // Default to no-op
	}

	for _, opt := range opts {
		opt(t)
	}

	return t
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

// CallStructured executes the PDF export operation and returns a structured result.
func (t *ExportToPDFTool) CallStructured(ctx context.Context, input string) (*types.ToolResult, error) {
	const op serrors.Op = "ExportToPDFTool.Call"

	params, err := agents.ParseToolInput[pdfExportInput](input)
	if err != nil {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeInvalidRequest),
				Message: fmt.Sprintf("failed to parse input: %v", err),
				Hints:   []string{tools.HintCheckRequiredFields, "Provide html parameter with content to convert"},
			},
		}, nil
	}

	if params.HTML == "" {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeInvalidRequest),
				Message: "html parameter is required",
				Hints:   []string{tools.HintCheckRequiredFields, "Provide HTML content to convert to PDF"},
			},
		}, nil
	}

	filename := params.Filename
	if filename == "" {
		filename = "export.pdf"
	}

	pdfData, err := t.convertHTMLToPDF(ctx, params.HTML, params.Landscape)
	if err != nil {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeServiceUnavailable),
				Message: fmt.Sprintf("PDF conversion failed: %v", err),
				Hints:   []string{tools.HintServiceMayBeDown, "Verify Gotenberg service is running", tools.HintRetryLater},
			},
		}, serrors.E(op, err, "PDF conversion failed")
	}

	url := fmt.Sprintf("/exports/%s", filename) // Fallback URL
	if t.storage != nil {
		metadata := storage.FileMetadata{
			ContentType: "application/pdf",
			Size:        int64(len(pdfData)),
		}
		savedURL, err := t.storage.Save(ctx, filename, bytes.NewReader(pdfData), metadata)
		if err != nil {
			return &types.ToolResult{
				CodecID: types.CodecToolError,
				Payload: types.ToolErrorPayload{
					Code:    string(tools.ErrCodeServiceUnavailable),
					Message: fmt.Sprintf("failed to save PDF file: %v", err),
					Hints:   []string{"File system may be full or permissions issue", tools.HintRetryLater},
				},
			}, serrors.E(op, err, "failed to save PDF file")
		}
		url = savedURL
	}
	url = resolveDownloadURL(ctx, url)

	response := pdfExportOutput{
		URL:      url,
		Filename: filename,
		Size:     int64(len(pdfData)),
	}

	return &types.ToolResult{
		CodecID: types.CodecJSON,
		Payload: types.JSONPayload{Output: response},
		Artifacts: []types.ToolArtifact{
			{
				Type:      "export",
				Name:      filename,
				MimeType:  "application/pdf",
				URL:       url,
				SizeBytes: int64(len(pdfData)),
			},
		},
	}, nil
}

// Call executes the PDF export operation.
func (t *ExportToPDFTool) Call(ctx context.Context, input string) (string, error) {
	return tools.FormatStructuredResult(t.CallStructured(ctx, input))
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
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.logger.Error(ctx, "failed to close response body", map[string]any{
				"error": closeErr.Error(),
			})
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
	storage      storage.FileStorage
}

// NewGotenbergPDFExporter creates a new Gotenberg PDF exporter.
//
// Parameters:
//   - gotenbergURL: Base URL of the Gotenberg service
//   - outputDir: Legacy parameter (deprecated, use storage instead)
//   - baseURL: Legacy parameter (deprecated, use storage URL instead)
//
// Deprecated: Use NewExportToPDFTool with storage.LocalFileStorage instead.
func NewGotenbergPDFExporter(gotenbergURL, outputDir, baseURL string) PDFExporter {
	// Create storage backend from legacy parameters
	fileStorage, err := storage.NewLocalFileStorage(outputDir, baseURL)
	if err != nil {
		// Fallback to no-op storage if directory creation fails
		fileStorage = storage.NewNoOpFileStorage()
	}

	return &GotenbergPDFExporter{
		gotenbergURL: gotenbergURL,
		outputDir:    outputDir,
		baseURL:      baseURL,
		storage:      fileStorage,
	}
}

// ExportToPDF exports HTML content to a PDF file.
func (e *GotenbergPDFExporter) ExportToPDF(ctx context.Context, html string, filename string, landscape bool) (string, error) {
	const op serrors.Op = "GotenbergPDFExporter.ExportToPDF"

	tool := NewExportToPDFTool(e.gotenbergURL, e.storage)
	pdfTool := tool.(*ExportToPDFTool)

	// Convert HTML to PDF
	pdfData, err := pdfTool.convertHTMLToPDF(ctx, html, landscape)
	if err != nil {
		return "", serrors.E(op, err)
	}

	// Save PDF file using storage
	if e.storage != nil {
		metadata := storage.FileMetadata{
			ContentType: "application/pdf",
			Size:        int64(len(pdfData)),
		}
		url, err := e.storage.Save(ctx, filename, bytes.NewReader(pdfData), metadata)
		if err != nil {
			return "", serrors.E(op, err, "failed to save PDF file")
		}
		return url, nil
	}

	// Fallback to legacy URL construction (no actual storage)
	url := fmt.Sprintf("%s/%s", e.baseURL, filename)
	return url, nil
}
