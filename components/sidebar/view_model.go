package sidebar

import (
	"context"

	"github.com/a-h/templ"
)

type NavNodeKind string

const (
	NavNodeKindLeaf   NavNodeKind = "leaf"
	NavNodeKindBranch NavNodeKind = "branch"
)

type NavNode struct {
	ID       string
	Kind     NavNodeKind
	Text     string
	Href     string
	Icon     templ.Component
	IsBeta   bool
	IsActive bool
	Children []NavNode
}

type NavTab struct {
	Label  string
	Value  string
	IsBeta bool
	Nodes  []NavNode
}

func BuildSidebarNavTabs(ctx context.Context, groups TabGroupCollection) []NavTab {
	tabs := make([]NavTab, 0, len(groups.Groups))
	for _, group := range groups.Groups {
		tabs = append(tabs, NavTab{
			Label:  group.Label,
			Value:  group.Value,
			IsBeta: group.IsBeta,
			Nodes:  buildSidebarNavNodes(ctx, group.Items),
		})
	}
	return tabs
}

func buildSidebarNavNodes(ctx context.Context, items []Item) []NavNode {
	nodes := make([]NavNode, 0, len(items))
	for _, item := range items {
		if item.IsLink() {
			link := asLink(item)
			nodes = append(nodes, NavNode{
				Kind:     NavNodeKindLeaf,
				Text:     link.Text(),
				Href:     link.Href(),
				Icon:     link.Icon(),
				IsBeta:   link.IsBeta(),
				IsActive: link.IsActive(ctx),
			})
			continue
		}

		group := asGroup(item)
		nodes = append(nodes, NavNode{
			ID:       group.ID(),
			Kind:     NavNodeKindBranch,
			Text:     group.Text(),
			Icon:     group.Icon(),
			IsBeta:   group.IsBeta(),
			IsActive: group.IsActive(ctx),
			Children: buildSidebarNavNodes(ctx, group.Children()),
		})
	}

	return nodes
}
