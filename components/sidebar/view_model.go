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
	return buildSidebarNavNodesForActivePath(ctx, items, true)
}

// buildSidebarNavNodesForActivePath resolves the active item once for each
// collection. A parent route (for example, /claims) can therefore keep its
// descendants active without also highlighting a more-specific sibling such as
// /claims/archive.
func buildSidebarNavNodesForActivePath(ctx context.Context, items []Item, onActivePath bool) []NavNode {
	nodes := make([]NavNode, 0, len(items))
	activeItemIndex := -1
	if onActivePath {
		activeItemIndex = mostSpecificActiveItemIndex(ctx, items)
	}

	for index, item := range items {
		isActive := index == activeItemIndex
		if item.IsLink() {
			link := asLink(item)
			nodes = append(nodes, NavNode{
				Kind:     NavNodeKindLeaf,
				Text:     link.Text(),
				Href:     link.Href(),
				Icon:     link.Icon(),
				IsBeta:   link.IsBeta(),
				IsActive: isActive,
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
			IsActive: isActive,
			Children: buildSidebarNavNodesForActivePath(ctx, group.Children(), isActive),
		})
	}

	return nodes
}

// mostSpecificActiveItemIndex returns the matching item whose descendant link
// has the longest href. This lets nested routes select their dedicated sidebar
// entry over a broader sibling route.
func mostSpecificActiveItemIndex(ctx context.Context, items []Item) int {
	activeItemIndex := -1
	longestMatch := -1
	for index, item := range items {
		matchLength := activeMatchLength(ctx, item)
		if matchLength > longestMatch {
			longestMatch = matchLength
			activeItemIndex = index
		}
	}

	return activeItemIndex
}

func activeMatchLength(ctx context.Context, item Item) int {
	if item.IsLink() {
		link := asLink(item)
		if link.IsActive(ctx) {
			return len(link.Href())
		}
		return -1
	}

	longestMatch := -1
	for _, child := range asGroup(item).Children() {
		if matchLength := activeMatchLength(ctx, child); matchLength > longestMatch {
			longestMatch = matchLength
		}
	}
	return longestMatch
}
