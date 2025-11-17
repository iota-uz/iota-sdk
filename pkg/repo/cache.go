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
			_, _ = fmt.Fprint(h, x)
		}
	}

	return hex.EncodeToString(h.Sum(nil))
}

func NewContextWithCache(ctx context.Context, cache Cache) context.Context {
	return context.WithValue(ctx, cacheKey{}, cache)
}

func GetCacheFromContext(ctx context.Context) (Cache, bool) {
	cache, ok := ctx.Value(cacheKey{}).(Cache)
	return cache, ok
}
