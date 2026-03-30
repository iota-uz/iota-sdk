// Package spotlight provides this package.
package spotlight

import (
	"context"
	"strings"

	"github.com/sirupsen/logrus"
)

type Grouper interface {
	Group(ctx context.Context, req SearchRequest, hits []SearchHit) SearchResponse
}

type DefaultGrouper struct{}

func NewDefaultGrouper() *DefaultGrouper {
	return &DefaultGrouper{}
}

// Group returns the grouped entities for spotlight lookups.
// user, group, role, client, project, order, report.
func (g *DefaultGrouper) Group(_ context.Context, _ SearchRequest, hits []SearchHit) SearchResponse {
	resp := SearchResponse{
		Navigate:  make([]SearchHit, 0),
		Data:      make([]SearchHit, 0),
		Knowledge: make([]SearchHit, 0),
		Other:     make([]SearchHit, 0),
		Groups:    make([]SearchGroup, 0, 4),
	}
	buckets := map[ResultDomain][]SearchHit{
		ResultDomainNavigate:  {},
		ResultDomainLookup:    {},
		ResultDomainKnowledge: {},
		ResultDomainAction:    {},
		ResultDomainOther:     {},
	}
	for _, hit := range hits {
		domain := normalizeDomain(hit.Document.Domain, hit.Document.EntityType)
		buckets[domain] = append(buckets[domain], hit)

		switch domain {
		case ResultDomainNavigate:
			resp.Navigate = append(resp.Navigate, hit)
		case ResultDomainKnowledge:
			resp.Knowledge = append(resp.Knowledge, hit)
		case ResultDomainLookup, ResultDomainAction:
			resp.Data = append(resp.Data, hit)
		case ResultDomainOther:
			logrus.WithFields(logrus.Fields{
				"entity_type": hit.Document.EntityType,
				"document_id": hit.Document.ID,
			}).Debug("spotlight unknown entity type grouped as other")
			resp.Other = append(resp.Other, hit)
		}
	}

	groupOrder := []ResultDomain{
		ResultDomainNavigate,
		ResultDomainLookup,
		ResultDomainKnowledge,
		ResultDomainAction,
		ResultDomainOther,
	}
	for _, domain := range groupOrder {
		if len(buckets[domain]) == 0 {
			continue
		}
		resp.Groups = append(resp.Groups, SearchGroup{
			Domain: domain,
			Title:  groupTitle(domain),
			Hits:   buckets[domain],
		})
	}
	return resp
}

func normalizeDomain(domain ResultDomain, entityType string) ResultDomain {
	if domain != "" {
		return domain
	}
	t := strings.ToLower(strings.TrimSpace(entityType))
	switch t {
	case "route", "page", "navigation", "quick_link":
		return ResultDomainNavigate
	case "knowledge", "kb", "doc", "docs":
		return ResultDomainKnowledge
	case "action", "command":
		return ResultDomainAction
	case "user", "group", "role", "client", "project", "order", "report", "policy", "vehicle", "organization":
		return ResultDomainLookup
	default:
		return ResultDomainOther
	}
}

func groupTitle(domain ResultDomain) string {
	switch domain {
	case ResultDomainNavigate:
		return "Spotlight.Group.Navigate"
	case ResultDomainLookup:
		return "Spotlight.Group.Data"
	case ResultDomainKnowledge:
		return "Spotlight.Group.Knowledge"
	case ResultDomainAction:
		return "Spotlight.Group.Actions"
	case ResultDomainOther:
		return "Spotlight.Group._Other"
	}
	return "Spotlight.Group._Other"
}
