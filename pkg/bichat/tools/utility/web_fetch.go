package utility

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/storage"
	tools "github.com/iota-uz/iota-sdk/pkg/bichat/tools"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

const (
	defaultWebFetchMaxDownloadBytes int64         = 20 << 20 // 20 MiB
	defaultWebFetchTimeout          time.Duration = 20 * time.Second
	maxSniffBytes                   int           = 512
)

// WebFetchTool fetches public image/PDF URLs and optionally saves them as artifacts.
type WebFetchTool struct {
	fileStorage      storage.FileStorage
	httpClient       *http.Client
	maxDownloadBytes int64
	timeout          time.Duration
}

// WebFetchToolOption configures WebFetchTool.
type WebFetchToolOption func(*WebFetchTool)

// WithWebFetchStorage enables save_to_artifacts by wiring file storage.
func WithWebFetchStorage(fs storage.FileStorage) WebFetchToolOption {
	return func(t *WebFetchTool) {
		t.fileStorage = fs
	}
}

// WithWebFetchHTTPClient sets a custom HTTP client (useful for testing).
func WithWebFetchHTTPClient(client *http.Client) WebFetchToolOption {
	return func(t *WebFetchTool) {
		if client != nil {
			t.httpClient = client
		}
	}
}

// WithWebFetchMaxDownloadBytes sets the maximum response body size.
func WithWebFetchMaxDownloadBytes(maxBytes int64) WebFetchToolOption {
	return func(t *WebFetchTool) {
		if maxBytes > 0 {
			t.maxDownloadBytes = maxBytes
		}
	}
}

// WithWebFetchTimeout sets request timeout for web fetch operations.
func WithWebFetchTimeout(timeout time.Duration) WebFetchToolOption {
	return func(t *WebFetchTool) {
		if timeout > 0 {
			t.timeout = timeout
		}
	}
}

const maxRedirects = 5

// NewWebFetchTool creates a new web fetch tool.
func NewWebFetchTool(opts ...WebFetchToolOption) agents.Tool {
	tool := &WebFetchTool{
		httpClient: &http.Client{
			Transport: newSSRFSafeTransport(),
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= maxRedirects {
					return fmt.Errorf("too many redirects")
				}
				if _, err := validatePublicWebFetchURL(req.URL.String()); err != nil {
					return fmt.Errorf("redirect blocked: %w", err)
				}
				return nil
			},
		},
		maxDownloadBytes: defaultWebFetchMaxDownloadBytes,
		timeout:          defaultWebFetchTimeout,
	}
	for _, opt := range opts {
		opt(tool)
	}
	return tool
}

// newSSRFSafeTransport returns an http.Transport whose DialContext validates
// resolved IPs so hostnames that resolve to private/internal IPs are rejected.
// Timeouts are set so stalled TLS handshakes or delayed headers don't hold connections indefinitely.
func newSSRFSafeTransport() *http.Transport {
	return &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("invalid address: %w", err)
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("DNS resolution failed: %w", err)
			}
			for _, ipa := range ips {
				if !isPublicIP(ipa.IP) {
					return nil, fmt.Errorf("resolved IP %s is not public", ipa.IP)
				}
			}
			dialer := &net.Dialer{Timeout: 10 * time.Second}
			return dialer.DialContext(ctx, network, net.JoinHostPort(host, port))
		},
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

// Name returns the tool name.
func (t *WebFetchTool) Name() string { return "web_fetch" }

// Description returns the tool description for the LLM.
func (t *WebFetchTool) Description() string {
	return "Fetch a single public URL for image/* or application/pdf content. " +
		"Use this to bring web images/PDFs into model context. " +
		"Set save_to_artifacts=true only when the user wants the file saved for later download/reference."
}

// Parameters returns the JSON Schema for tool parameters.
func (t *WebFetchTool) Parameters() map[string]any {
	return agents.ToolSchema[webFetchInput]()
}

type webFetchInput struct {
	URL             string `json:"url" jsonschema:"description=Public http/https URL to fetch (supports image/* and application/pdf)"`
	SaveToArtifacts bool   `json:"save_to_artifacts,omitempty" jsonschema:"description=When true, download and save the fetched file as an artifact;default=false"`
	Filename        string `json:"filename,omitempty" jsonschema:"description=Optional filename to use when save_to_artifacts is true"`
}

type webFetchOutput struct {
	SourceURL     string `json:"source_url"`
	ContentType   string `json:"content_type"`
	SizeBytes     int64  `json:"size_bytes"`
	Injectable    bool   `json:"injectable"`
	InjectionType string `json:"injection_type,omitempty"`
	InjectionURL  string `json:"injection_url,omitempty"`
	Saved         bool   `json:"saved"`
	SavedURL      string `json:"saved_url,omitempty"`
	Filename      string `json:"filename,omitempty"`
}

// CallStructured executes web fetch and returns structured output.
func (t *WebFetchTool) CallStructured(ctx context.Context, input string) (*types.ToolResult, error) {
	params, err := agents.ParseToolInput[webFetchInput](input)
	if err != nil {
		return webFetchToolError(
			tools.ErrCodeInvalidRequest,
			fmt.Sprintf("failed to parse input: %v", err),
			tools.HintCheckRequiredFields,
			tools.HintCheckFieldFormat,
		), nil
	}

	targetURL, err := validatePublicWebFetchURL(params.URL)
	if err != nil {
		return webFetchToolError(
			tools.ErrCodePolicyViolation,
			err.Error(),
			"Use a public http/https URL (no localhost/private/internal addresses)",
		), nil
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(timeoutCtx, http.MethodGet, targetURL.String(), nil)
	if err != nil {
		return webFetchToolError(
			tools.ErrCodeInvalidRequest,
			fmt.Sprintf("failed to build request: %v", err),
			tools.HintCheckFieldFormat,
		), nil
	}
	req.Header.Set("Accept", "image/*,application/pdf;q=0.9,*/*;q=0.1")
	req.Header.Set("User-Agent", "iota-sdk/web_fetch")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return webFetchToolError(
			tools.ErrCodeServiceUnavailable,
			fmt.Sprintf("failed to fetch URL: %v", err),
			tools.HintCheckConnection,
			tools.HintRetryLater,
		), nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return webFetchToolError(
			tools.ErrCodeServiceUnavailable,
			fmt.Sprintf("fetch failed with status %d", resp.StatusCode),
			"Ensure the URL is accessible and returns a successful response",
			tools.HintRetryLater,
		), nil
	}

	body, tooLarge, err := readBodyWithCap(resp.Body, t.maxDownloadBytes)
	if err != nil {
		return webFetchToolError(
			tools.ErrCodeServiceUnavailable,
			fmt.Sprintf("failed to read response body: %v", err),
			tools.HintRetryLater,
		), nil
	}
	if tooLarge {
		return webFetchToolError(
			tools.ErrCodeDataTooLarge,
			fmt.Sprintf("response exceeds max size of %d bytes", t.maxDownloadBytes),
			"Fetch a smaller file or increase tool max download size in runtime config",
		), nil
	}
	if len(body) == 0 {
		return webFetchToolError(
			tools.ErrCodeNoData,
			"fetched response is empty",
			"Try another URL that contains an image or PDF file",
		), nil
	}

	contentType, sniffedType := resolveSupportedContentType(resp.Header.Get("Content-Type"), body)
	if contentType == "" {
		msg := "unsupported content type"
		if sniffedType != "" {
			msg = fmt.Sprintf("unsupported content type: %s", sniffedType)
		}
		return webFetchToolError(
			tools.ErrCodeInvalidRequest,
			msg,
			"Supported types are image/* and application/pdf only",
		), nil
	}

	filename := resolveWebFetchFilename(params.Filename, targetURL, contentType)
	injectionType := injectionTypeForContentType(contentType)

	result := webFetchOutput{
		SourceURL:     targetURL.String(),
		ContentType:   contentType,
		SizeBytes:     int64(len(body)),
		Injectable:    true,
		InjectionType: injectionType,
		InjectionURL:  targetURL.String(),
		Saved:         false,
		Filename:      filename,
	}

	if !params.SaveToArtifacts {
		return &types.ToolResult{
			CodecID: types.CodecJSON,
			Payload: types.JSONPayload{Output: result},
		}, nil
	}

	if t.fileStorage == nil {
		return webFetchToolError(
			tools.ErrCodeServiceUnavailable,
			"save_to_artifacts=true but storage is not configured",
			"Disable save_to_artifacts or configure WithWebFetchStorage at runtime",
		), nil
	}

	saveCtx, saveCancel := context.WithTimeout(ctx, t.timeout)
	defer saveCancel()
	savedURL, err := t.fileStorage.Save(saveCtx, filename, bytes.NewReader(body), storage.FileMetadata{
		ContentType: contentType,
		Size:        int64(len(body)),
	})
	if err != nil {
		return webFetchToolError(
			tools.ErrCodeServiceUnavailable,
			fmt.Sprintf("failed to save artifact: %v", err),
			tools.HintRetryLater,
		), nil
	}

	result.Saved = true
	result.SavedURL = savedURL
	result.InjectionURL = savedURL

	return &types.ToolResult{
		CodecID: types.CodecJSON,
		Payload: types.JSONPayload{Output: result},
		Artifacts: []types.ToolArtifact{
			{
				Type:      "export",
				Name:      filename,
				MimeType:  contentType,
				URL:       savedURL,
				SizeBytes: int64(len(body)),
				Metadata: map[string]any{
					"source_url": targetURL.String(),
				},
			},
		},
	}, nil
}

// Call executes web fetch operation.
func (t *WebFetchTool) Call(ctx context.Context, input string) (string, error) {
	return tools.FormatStructuredResult(t.CallStructured(ctx, input))
}

func webFetchToolError(code tools.ToolErrorCode, message string, hints ...string) *types.ToolResult {
	return &types.ToolResult{
		CodecID: types.CodecToolError,
		Payload: types.ToolErrorPayload{
			Code:    string(code),
			Message: message,
			Hints:   hints,
		},
	}
}

func validatePublicWebFetchURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("url is required")
	}

	parsed, err := url.ParseRequestURI(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %v", err)
	}

	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("only http/https URLs are allowed")
	}

	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return nil, fmt.Errorf("URL host is required")
	}
	if isBlockedHostname(host) {
		return nil, fmt.Errorf("URL host is not allowed by public-only policy")
	}

	if ip := net.ParseIP(host); ip != nil && !isPublicIP(ip) {
		return nil, fmt.Errorf("URL resolves to non-public IP address")
	}

	return parsed, nil
}

func isBlockedHostname(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	if strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".localdomain") || strings.HasSuffix(host, ".internal") {
		return true
	}
	return false
}

func isPublicIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	if !ip.IsGlobalUnicast() {
		return false
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		// RFC 6598 Carrier-Grade NAT: 100.64.0.0/10
		if ipv4[0] == 100 && ipv4[1]&0xC0 == 0x40 {
			return false
		}
	}
	return true
}

func readBodyWithCap(r io.Reader, maxBytes int64) ([]byte, bool, error) {
	if maxBytes <= 0 {
		maxBytes = defaultWebFetchMaxDownloadBytes
	}
	lr := io.LimitReader(r, maxBytes+1)
	body, err := io.ReadAll(lr)
	if err != nil {
		return nil, false, err
	}
	if int64(len(body)) > maxBytes {
		return nil, true, nil
	}
	return body, false, nil
}

func resolveSupportedContentType(contentTypeHeader string, body []byte) (string, string) {
	headerType := strings.ToLower(strings.TrimSpace(contentTypeHeader))
	if headerType != "" {
		if mediaType, _, err := mime.ParseMediaType(headerType); err == nil {
			headerType = strings.ToLower(strings.TrimSpace(mediaType))
		}
	}

	if isSupportedWebFetchContentType(headerType) {
		return headerType, headerType
	}

	sniffed := strings.ToLower(strings.TrimSpace(http.DetectContentType(bodyPrefix(body))))
	if isSupportedWebFetchContentType(sniffed) {
		return sniffed, sniffed
	}

	if headerType != "" {
		return "", headerType
	}
	return "", sniffed
}

func bodyPrefix(body []byte) []byte {
	if len(body) <= maxSniffBytes {
		return body
	}
	return body[:maxSniffBytes]
}

func isSupportedWebFetchContentType(contentType string) bool {
	return strings.HasPrefix(contentType, "image/") || contentType == "application/pdf"
}

func injectionTypeForContentType(contentType string) string {
	if contentType == "application/pdf" {
		return "input_file"
	}
	return "input_image"
}

func resolveWebFetchFilename(requested string, sourceURL *url.URL, contentType string) string {
	filename := sanitizeWebFetchFilename(requested)
	if filename == "" && sourceURL != nil {
		filename = sanitizeWebFetchFilename(path.Base(sourceURL.Path))
	}
	if filename == "" {
		filename = "web_fetch" + defaultWebFetchExtension(contentType)
	}
	if path.Ext(filename) == "" {
		filename += defaultWebFetchExtension(contentType)
	}
	return filename
}

func sanitizeWebFetchFilename(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	base := path.Base(name)
	if base == "." || base == "/" || base == "" {
		return ""
	}
	return base
}

func defaultWebFetchExtension(contentType string) string {
	if contentType == "application/pdf" {
		return ".pdf"
	}
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/svg+xml":
		return ".svg"
	default:
		return ".img"
	}
}
