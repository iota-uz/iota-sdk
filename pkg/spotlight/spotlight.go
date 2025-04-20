// Package spotlight is a package that provides a way to show a list of items in a spotlight.
package spotlight

import (
	"context"
	"sort"
	"sync"

	"github.com/lithammer/fuzzysearch/fuzzy"
)

// DataSource provides external items for Spotlight.
type DataSource interface {
	Find(ctx context.Context, q string) []Item
}

// Spotlight streams items matching a query over a channel.
type Spotlight interface {
	Find(ctx context.Context, q string) <-chan Item
	Register(...Item)
	RegisterDataSource(DataSource)
}

// New creates a Spotlight without result limits.
func New() Spotlight {
	return &spotlight{
		items:       []Item{},
		dataSources: []DataSource{},
	}
}

type spotlight struct {
	items       []Item
	dataSources []DataSource
}

func (s *spotlight) Register(i ...Item) {
	s.items = append(s.items, i...)
}

func (s *spotlight) RegisterDataSource(ds DataSource) {
	s.dataSources = append(s.dataSources, ds)
}

func (s *spotlight) Find(ctx context.Context, q string) <-chan Item {
	in := make(chan Item)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		words := make([]string, len(s.items))
		for i, it := range s.items {
			words[i] = it.Label(ctx)
		}
		ranks := fuzzy.RankFindNormalizedFold(q, words)
		sort.Sort(ranks)
		for _, rank := range ranks {
			select {
			case <-ctx.Done():
				return
			case in <- s.items[rank.OriginalIndex]:
			}
		}
	}()

	wg.Add(len(s.dataSources))
	for _, ds := range s.dataSources {
		ds := ds
		go func() {
			defer wg.Done()
			items := ds.Find(ctx, q)
			for _, item := range items {
				select {
				case <-ctx.Done():
					return
				case in <- item:
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(in)
	}()

	return in
}
