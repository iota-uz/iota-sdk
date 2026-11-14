package pkgdocs

import "sync"

// cacheBytes bounds the memory of the document cache.
const cacheBytes = 8 << 20

// textCache keeps the text of documents by artifact hash and file. An artifact
// is immutable, so an entry never goes stale; the cache is only bounded: when
// it is full it starts over.
type textCache struct {
	mu    sync.Mutex
	max   int
	size  int
	items map[string][]byte
}

func newTextCache(limit int) *textCache { return &textCache{max: limit, items: map[string][]byte{}} }

func (c *textCache) get(hash, file string, load func() ([]byte, error)) ([]byte, error) {
	key := hash + "\x00" + file
	c.mu.Lock()
	if b, ok := c.items[key]; ok {
		c.mu.Unlock()
		return b, nil
	}
	c.mu.Unlock()
	b, err := load()
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.size+len(b) > c.max {
		c.items, c.size = map[string][]byte{}, 0
	}
	if len(b) <= c.max {
		c.items[key] = b
		c.size += len(b)
	}
	return b, nil
}
