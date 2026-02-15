package spotlight

import (
	"context"
	"strings"
)

type Grouper interface {
	Group(ctx context.Context, req SearchRequest, hits []SearchHit) SearchResponse
}

type DefaultGrouper struct{}

func NewDefaultGrouper() *DefaultGrouper {
	return &DefaultGrouper{}
}

func (g *DefaultGrouper) Group(_ context.Context, _ SearchRequest, hits []SearchHit) SearchResponse {
	resp := SearchResponse{
		Navigate:  make([]SearchHit, 0),
		Data:      make([]SearchHit, 0),
		Knowledge: make([]SearchHit, 0),
		Other:     make([]SearchHit, 0),
	}
	for _, hit := range hits {
		t := strings.ToLower(strings.TrimSpace(hit.Document.EntityType))
		switch t {
		case "route", "page", "navigation", "quick_link":
			resp.Navigate = append(resp.Navigate, hit)
		case "knowledge", "kb", "doc", "docs":
			resp.Knowledge = append(resp.Knowledge, hit)
		case "user", "group", "role", "client", "project", "order", "report":
			resp.Data = append(resp.Data, hit)
		default:
			resp.Other = append(resp.Other, hit)
		}
	}
	return resp
}
