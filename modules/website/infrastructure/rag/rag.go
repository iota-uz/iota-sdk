// Package rag provides this package.
package rag

import "context"

type Provider interface {
	SearchRelevantContext(ctx context.Context, query string) ([]string, error)
}
