// Package spotlight is a package that provides a way to show a list of items in a spotlight.
package spotlight

import (
	"context"
	"sync"
)

// DataSource provides external items for Spotlight.
type DataSource interface {
	Find(ctx context.Context, q string) []Item
}

// Spotlight streams items matching a query over a channel.
type Spotlight interface {
	Find(ctx context.Context, q string) <-chan Item
	Register(ds DataSource)
}

func New() Spotlight {
	return &spotlight{
		dataSources: []DataSource{},
	}
}

type spotlight struct {
	dataSources []DataSource
}

func (s *spotlight) Register(ds DataSource) {
	s.dataSources = append(s.dataSources, ds)
}

func (s *spotlight) Find(ctx context.Context, q string) <-chan Item {
	in := make(chan Item)

	var wg sync.WaitGroup

	wg.Add(len(s.dataSources))
	for _, ds := range s.dataSources {
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
