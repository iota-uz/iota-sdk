package spotlight

func ToViewResponse(resp SearchResponse) ViewResponse {
	groups := make([]ViewGroup, 0, 4)
	if len(resp.Navigate) > 0 {
		groups = append(groups, ViewGroup{Key: "navigate", Title: "Navigate", Hits: resp.Navigate})
	}
	if len(resp.Data) > 0 {
		groups = append(groups, ViewGroup{Key: "data", Title: "Data", Hits: resp.Data})
	}
	if len(resp.Knowledge) > 0 {
		groups = append(groups, ViewGroup{Key: "knowledge", Title: "Knowledge", Hits: resp.Knowledge})
	}
	if len(resp.Other) > 0 {
		groups = append(groups, ViewGroup{Key: "other", Title: "Other", Hits: resp.Other})
	}

	view := ViewResponse{Groups: groups}
	if resp.Agent != nil {
		view.Agent = &ViewAgent{Summary: resp.Agent.Summary, Actions: resp.Agent.Actions}
	}
	return view
}
