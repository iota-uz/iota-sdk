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
		if domain == ResultDomainOther {
			logrus.WithFields(logrus.Fields{
				"entity_type": hit.Document.EntityType,
				"document_id": hit.Document.ID,
			}).Debug("spotlight unknown entity type grouped as other")
		}
	}

	resp := SearchResponse{Groups: make([]SearchGroup, 0, 4)}
	groupOrder := []ResultDomain{
		ResultDomainNavigate,
		ResultDomainLookup,
		ResultDomainKnowledge,
		ResultDomainAction,
		ResultDomainOther,
	}
	for _, domain := range groupOrder {
		hitsForDomain := buckets[domain]
		if len(hitsForDomain) == 0 {
			continue
		}
		group := buildSearchGroup(domain, hitsForDomain)
		resp.Groups = append(resp.Groups, group)
	}
	return resp
}

func buildSearchGroup(domain ResultDomain, hits []SearchHit) SearchGroup {
	key, title, titleKey := defaultGroupMeta(domain)
	for _, hit := range hits {
		if metadataKey, metadataTitle, metadataTitleKey, ok := documentGroupMeta(hit.Document); ok {
			key, title, titleKey = metadataKey, metadataTitle, metadataTitleKey
			break
		}
	}
	return SearchGroup{
		Key:      key,
		Domain:   domain,
		Title:    title,
		TitleKey: titleKey,
		Hits:     hits,
	}
}

func documentGroupMeta(doc SearchDocument) (key, title, titleKey string, ok bool) {
	key = strings.TrimSpace(doc.Metadata["group_key"])
	if key == "" {
		return "", "", "", false
	}
	title = strings.TrimSpace(doc.Metadata["group_title"])
	titleKey = strings.TrimSpace(doc.Metadata["group_title_key"])
	return key, title, titleKey, true
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
	case "":
		return ResultDomainOther
	default:
		return ResultDomainLookup
	}
}

func defaultGroupMeta(domain ResultDomain) (key, title, titleKey string) {
	switch domain {
	case ResultDomainNavigate:
		return "navigate", "", "Spotlight.Group.Navigate"
	case ResultDomainLookup:
		return "data", "", "Spotlight.Group.Data"
	case ResultDomainKnowledge:
		return "knowledge", "", "Spotlight.Group.Knowledge"
	case ResultDomainAction:
		return "actions", "", "Spotlight.Group.Actions"
	case ResultDomainOther:
		return "other", "", "Spotlight.Group._Other"
	}
	return "other", "", "Spotlight.Group._Other"
}
