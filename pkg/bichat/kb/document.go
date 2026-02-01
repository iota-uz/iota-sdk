package kb

import "time"

// DocumentType represents the type of document being indexed.
type DocumentType string

const (
	// DocumentTypeMarkdown represents markdown documents
	DocumentTypeMarkdown DocumentType = "markdown"
	// DocumentTypeHTML represents HTML documents
	DocumentTypeHTML DocumentType = "html"
	// DocumentTypePDF represents PDF documents
	DocumentTypePDF DocumentType = "pdf"
	// DocumentTypeText represents plain text documents
	DocumentTypeText DocumentType = "text"
	// DocumentTypeCode represents source code files
	DocumentTypeCode DocumentType = "code"
	// DocumentTypeJSON represents JSON documents
	DocumentTypeJSON DocumentType = "json"
)

// Document represents a document to be indexed in the knowledge base.
// Documents are content-addressable, with ID serving as the primary key.
type Document struct {
	// ID is the unique identifier for the document
	ID string `json:"id"`
	// Title is the document title or heading
	Title string `json:"title"`
	// Content is the full text content to be indexed
	Content string `json:"content"`
	// Path is the file path or URL of the document
	Path string `json:"path"`
	// Type is the document type (markdown, html, pdf, etc.)
	Type DocumentType `json:"type"`
	// Metadata contains additional key-value pairs for filtering and display
	Metadata map[string]string `json:"metadata"`
	// UpdatedAt is the last modification time
	UpdatedAt time.Time `json:"updated_at"`
}

// SearchResult represents a single search result with relevance scoring.
type SearchResult struct {
	// Document is the matching document
	Document Document
	// Score is the relevance score (higher is more relevant)
	Score float64
	// Excerpt is a highlighted snippet showing the match context
	Excerpt string
	// Fragments are additional matching fragments from the document
	Fragments []string
}

// SearchOptions configures search behavior and filtering.
type SearchOptions struct {
	// TopK is the maximum number of results to return (default: 10)
	TopK int
	// MinScore is the minimum relevance score threshold
	MinScore float64
	// IncludeTags filters results to only include documents with these tags
	IncludeTags []string
	// ExcludeTags filters out documents with these tags
	ExcludeTags []string
	// BoostFields contains field boost weights for relevance tuning
	// Higher values increase the importance of that field in scoring
	BoostFields map[string]float64
}

// ChangeType represents the type of document change event.
type ChangeType string

const (
	// ChangeTypeCreate indicates a new document was created
	ChangeTypeCreate ChangeType = "create"
	// ChangeTypeUpdate indicates an existing document was updated
	ChangeTypeUpdate ChangeType = "update"
	// ChangeTypeDelete indicates a document was deleted
	ChangeTypeDelete ChangeType = "delete"
)

// DocumentChange represents a change to a document in the knowledge base.
// Used by DocumentSource.Watch() to notify subscribers of updates.
type DocumentChange struct {
	// Type is the type of change (create, update, delete)
	Type ChangeType
	// Document is the affected document (nil for delete events)
	Document *Document
	// ID is the document ID (for delete events when Document is nil)
	ID string
}
