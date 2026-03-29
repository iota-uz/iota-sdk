// Package spotlight provides this package.
package spotlight

func ToViewResponse(resp SearchResponse) ViewResponse {
	groupOrder := make([]string, 0, max(4, len(resp.Groups)))
	groups := make(map[string]*ViewGroup, max(4, len(resp.Groups)))
	appendHit := func(hit SearchHit) {
		key, title, titleKey := viewGroupMeta(hit)
		group, ok := groups[key]
		if !ok {
			group = &ViewGroup{Key: key, Title: title, TitleKey: titleKey, Hits: make([]SearchHit, 0, 8)}
			groups[key] = group
			groupOrder = append(groupOrder, key)
		}
		group.Hits = append(group.Hits, hit)
	}

	if len(resp.Groups) > 0 {
		for _, group := range resp.Groups {
			for _, hit := range group.Hits {
				appendHit(hit)
			}
		}
	} else {
		for _, hit := range resp.Navigate {
			appendHit(hit)
		}
		for _, hit := range resp.Data {
			appendHit(hit)
		}
		for _, hit := range resp.Knowledge {
			appendHit(hit)
		}
		for _, hit := range resp.Other {
			appendHit(hit)
		}
	}

	view := ViewResponse{Groups: make([]ViewGroup, 0, len(groupOrder))}
	for _, key := range groupOrder {
		view.Groups = append(view.Groups, *groups[key])
	}
	if resp.Agent != nil {
		view.Agent = &ViewAgent{Summary: resp.Agent.Summary, Actions: resp.Agent.Actions}
	}
	return view
}

func viewGroupMeta(hit SearchHit) (string, string, string) {
	if key := hit.Document.Metadata["group_key"]; key != "" {
		if title := hit.Document.Metadata["group_title"]; title != "" {
			return key, title, ""
		}
		if titleKey := hit.Document.Metadata["group_title_key"]; titleKey != "" {
			return key, "", titleKey
		}
	}
	domain := normalizeDomain(hit.Document.Domain, hit.Document.EntityType)
	switch domain {
	case ResultDomainNavigate:
		return "navigate", "", "Spotlight.Group.Navigate"
	case ResultDomainKnowledge:
		return "knowledge", "", "Spotlight.Group.Knowledge"
	case ResultDomainAction:
		return "actions", "", "Spotlight.Group.Actions"
	case ResultDomainLookup:
		return "data", "", "Spotlight.Group.Data"
	default:
		return "other", "", "Spotlight.Group._Other"
	}
}
