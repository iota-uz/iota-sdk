package repo

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"math"
	"time"
)

type cacheKey struct{}

type Cache interface {
	// Get retrieves a value from the cache by key.
	Get(key string) (any, bool)
	// Set stores a value in the cache with the specified key.
	Set(key string, value any) error
	// Delete removes a value from the cache by key.
	Delete(key string)
	// Clear removes all values from the cache.
	Clear()
}

func CacheKey(keys ...interface{}) string {
	h := fnv.New64a()

	for _, v := range keys {
		switch x := v.(type) {
		case string:
			h.Write([]byte(x))
		case []byte:
			h.Write(x)
		case bool:
			if x {
				h.Write([]byte{1})
			} else {
				h.Write([]byte{0})
			}
		case byte:
			h.Write([]byte{x})
		case int:
			var buf [8]byte
			binary.LittleEndian.PutUint64(buf[:], uint64(x))
			h.Write(buf[:])
		case int8:
			h.Write([]byte{byte(x)})
		case int16:
			var buf [2]byte
			binary.LittleEndian.PutUint16(buf[:], uint16(x))
			h.Write(buf[:])
		case int32:
			var buf [4]byte
			binary.LittleEndian.PutUint32(buf[:], uint32(x))
			h.Write(buf[:])
		case int64:
			var buf [8]byte
			binary.LittleEndian.PutUint64(buf[:], uint64(x))
			h.Write(buf[:])
		case uint:
			var buf [8]byte
			binary.LittleEndian.PutUint64(buf[:], uint64(x))
			h.Write(buf[:])
		case uint16:
			var buf [2]byte
			binary.LittleEndian.PutUint16(buf[:], x)
			h.Write(buf[:])
		case uint32:
			var buf [4]byte
			binary.LittleEndian.PutUint32(buf[:], x)
			h.Write(buf[:])
		case uint64:
			var buf [8]byte
			binary.LittleEndian.PutUint64(buf[:], x)
			h.Write(buf[:])
		case uintptr:
			var buf [8]byte
			binary.LittleEndian.PutUint64(buf[:], uint64(x))
			h.Write(buf[:])
		case float32:
			var buf [4]byte
			binary.LittleEndian.PutUint32(buf[:], math.Float32bits(x))
			h.Write(buf[:])
		case float64:
			var buf [8]byte
			binary.LittleEndian.PutUint64(buf[:], math.Float64bits(x))
			h.Write(buf[:])
		case complex64:
			// hash real part then imaginary
			var buf [4]byte
			binary.LittleEndian.PutUint32(buf[:], math.Float32bits(real(x)))
			h.Write(buf[:])
			binary.LittleEndian.PutUint32(buf[:], math.Float32bits(imag(x)))
			h.Write(buf[:])
		case complex128:
			var buf [8]byte
			binary.LittleEndian.PutUint64(buf[:], math.Float64bits(real(x)))
			h.Write(buf[:])
			binary.LittleEndian.PutUint64(buf[:], math.Float64bits(imag(x)))
			h.Write(buf[:])
		case time.Time:
			if b, err := x.MarshalBinary(); err == nil {
				h.Write(b)
			} else {
				h.Write([]byte(x.String()))
			}
		default:
			h.Write([]byte(fmt.Sprint(x)))
		}
	}

	return hex.EncodeToString(h.Sum(nil))
}

func WithCache(ctx context.Context, cache Cache) context.Context {
	return context.WithValue(ctx, cacheKey{}, cache)
}

func UseCache(ctx context.Context) (Cache, bool) {
	cache := ctx.Value(cacheKey{})
	if cache == nil {
		return nil, false
	}

	c, ok := cache.(Cache)
	if !ok {
		return nil, false
	}
	return c, true
}

type InMemoryCache struct {
	// cache is a map of string to any type
	cache map[string]any
}

func NewInMemoryCache() Cache {
	return &InMemoryCache{
		cache: make(map[string]any),
	}
}

func (c *InMemoryCache) Get(key string) (any, bool) {
	if value, ok := c.cache[key]; ok {
		return value, true
	}
	return nil, false
}

func (c *InMemoryCache) Set(key string, value any) error {
	c.cache[key] = value
	return nil
}

func (c *InMemoryCache) Delete(key string) {
	delete(c.cache, key)
}

func (c *InMemoryCache) Clear() {
	for k := range c.cache {
		delete(c.cache, k)
	}
}
