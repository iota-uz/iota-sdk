# Knowledge Base Indexing Package

Full-text search and indexing for BI-Chat knowledge bases using Bleve.

## Overview

The `kb` package provides a flexible, extensible knowledge base indexing system with:

- **Full-text search** with relevance scoring
- **Highlighted snippets** showing match context
- **Multiple document sources** (filesystem, database, custom)
- **Live indexing** via file system watching
- **Field boosting** for relevance tuning
- **Tag-based filtering** for metadata queries

## Quick Start

### Basic Usage

```go
import "github.com/iota-uz/iota-sdk/pkg/bichat/kb"

// Create a new Bleve index
indexer, searcher, err := kb.NewBleveIndex("/path/to/index")
if err != nil {
    log.Fatal(err)
}
defer indexer.Close()

// Index a document
doc := kb.Document{
    ID:       "doc1",
    Title:    "Getting Started",
    Content:  "This is a guide to getting started...",
    Type:     kb.DocumentTypeMarkdown,
    Metadata: map[string]string{"category": "tutorial"},
}
err = indexer.IndexDocument(context.Background(), doc)

// Search
results, err := searcher.Search(context.Background(), "getting started", kb.SearchOptions{
    TopK: 10,
})
for _, result := range results {
    fmt.Printf("Score: %.2f | %s\n", result.Score, result.Document.Title)
    fmt.Printf("Excerpt: %s\n\n", result.Excerpt)
}
```

### Indexing from Filesystem

```go
import (
    "github.com/iota-uz/iota-sdk/pkg/bichat/kb"
    "github.com/iota-uz/iota-sdk/pkg/bichat/kb/sources"
)

// Create filesystem source
source := sources.NewFileSystemSource(
    "/path/to/docs",
    sources.WithExtensions(".md", ".txt"),
    sources.WithRecursive(true),
)

// Rebuild index from source
err := indexer.Rebuild(context.Background(), source)

// Watch for changes (live indexing)
changes, err := source.Watch(context.Background())
for change := range changes {
    switch change.Type {
    case kb.ChangeTypeCreate, kb.ChangeTypeUpdate:
        indexer.IndexDocument(ctx, *change.Document)
    case kb.ChangeTypeDelete:
        indexer.DeleteDocument(ctx, change.ID)
    }
}
```

### Custom Configuration

```go
indexer, searcher, err := kb.NewBleveIndex(
    "/path/to/index",
    kb.WithAnalyzer("standard"),
    kb.WithStopWords([]string{"the", "a", "an"}),
    kb.WithBoostConfig(map[string]float64{
        "title":   3.0,  // Boost title matches
        "content": 1.0,
    }),
    kb.WithFragmentSize(200),
    kb.WithMaxFragments(3),
)
```

### Advanced Search

```go
results, err := searcher.Search(ctx, "database optimization", kb.SearchOptions{
    TopK:     20,
    MinScore: 0.5,
    IncludeTags: []string{"tutorial", "advanced"},
    ExcludeTags: []string{"deprecated"},
    BoostFields: map[string]float64{
        "title": 2.0,
    },
})
```

## Architecture

### Core Interfaces

- **`KBIndexer`** - Builds and maintains the search index
- **`KBSearcher`** - Searches the index and retrieves documents
- **`DocumentSource`** - Provides documents from various sources

### Document Types

Supported document types:
- `DocumentTypeMarkdown` - Markdown files
- `DocumentTypeHTML` - HTML documents
- `DocumentTypePDF` - PDF files
- `DocumentTypeText` - Plain text
- `DocumentTypeCode` - Source code files
- `DocumentTypeJSON` - JSON documents

### Document Sources

#### FileSystemSource

Indexes files from a directory:

```go
source := sources.NewFileSystemSource(
    "/docs",
    sources.WithExtensions(".md", ".html"),
    sources.WithIgnorePatterns("*.tmp", ".git/*"),
    sources.WithRecursive(true),
    sources.WithExtractTitle(true),
    sources.WithIncludeMetadata(true),
)
```

#### DatabaseSource

Indexes from a database:

```go
type MyDocRepository struct {
    db *sql.DB
}

func (r *MyDocRepository) List(ctx context.Context) ([]kb.Document, error) {
    // Query database and return documents
}

func (r *MyDocRepository) Watch(ctx context.Context) (<-chan kb.DocumentChange, error) {
    // Optional: implement change notifications
    return nil, nil
}

source := sources.NewDatabaseSource(&MyDocRepository{db: db})
```

## Implementation Details

### Bleve Index

The default implementation uses [Bleve v2](https://blevesearch.com/) for full-text search:

- **Analyzer**: Standard analyzer (customizable)
- **Storage**: Bolt DB (embedded)
- **Query Language**: Bleve query syntax
- **Highlighting**: Automatic excerpt generation

### Thread Safety

All implementations are thread-safe:
- `KBIndexer` - Safe for concurrent indexing
- `KBSearcher` - Safe for concurrent searches
- Filesystem watcher - Single goroutine per source

### Performance

For optimal performance:
- Use `IndexDocuments()` for batch indexing
- Set appropriate `TopK` limits for searches
- Enable `WithFragmentSize()` to limit excerpt generation
- Use tag filters instead of post-processing results

## Testing

Run tests with:

```bash
go test ./pkg/bichat/kb/... -v
```

## Future Enhancements

Planned features:
- Vector embeddings for semantic search
- Multi-language support
- PDF text extraction
- HTML content stripping
- Faceted search
- Index sharding for large datasets
