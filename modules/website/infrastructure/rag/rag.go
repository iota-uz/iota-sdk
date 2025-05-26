package rag

import "context"

type Provider interface {
	SearchRelevantContext(ctx context.Context, query string) ([]string, error)
}
