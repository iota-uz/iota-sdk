package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
)

type MemoRequest struct {
	Namespace, DataScope string
	Input                any
	TTL                  time.Duration
}

// MemoizeJSON shares the canonical bounded store and singleflight domain with
// Lens execution. It handles typed report assembly that happens before a spec
// can be built; values are JSON round-tripped for clone safety.
func MemoizeJSON[T any](ctx context.Context, runtime *Runtime, req MemoRequest, compute func(context.Context) (T, error)) (T, error) {
	var zero T
	if runtime == nil {
		return zero, fmt.Errorf("lens runtime is required")
	}
	key, err := memoIdentity(runtime, req)
	if err != nil {
		return zero, err
	}
	if snapshot, ok := runtime.store.Load(ctx, key); ok {
		if decoded, decodeErr := decodeMemo[T](snapshot); decodeErr == nil {
			return decoded, nil
		}
	}
	value, err, _ := runtime.flights.Do(key, func() (any, error) {
		if snapshot, ok := runtime.store.Load(ctx, key); ok {
			if decoded, decodeErr := decodeMemo[T](snapshot); decodeErr == nil {
				return decoded, nil
			}
		}
		computed, computeErr := compute(ctx)
		if computeErr != nil {
			return nil, computeErr
		}
		data, marshalErr := json.Marshal(computed)
		if marshalErr != nil {
			return nil, marshalErr
		}
		fr, frameErr := frame.New("memo", frame.Field{Name: "json", Type: frame.FieldTypeString, Values: []any{string(data)}})
		if frameErr != nil {
			return nil, frameErr
		}
		frames, frameErr := frame.NewFrameSet(fr)
		if frameErr != nil {
			return nil, frameErr
		}
		ttl := req.TTL
		if ttl <= 0 {
			ttl = runtime.ttl
		}
		now := time.Now()
		runtime.store.Save(ctx, key, &ExecutionSnapshot{ID: key, DataScope: req.DataScope, Datasets: map[string]*frame.FrameSet{"memo": frames}, CreatedAt: now, ExpiresAt: now.Add(ttl)}, ttl)
		return computed, nil
	})
	if err != nil {
		return zero, err
	}
	result, ok := value.(T)
	if !ok {
		return zero, fmt.Errorf("memo %q returned unexpected type %T", req.Namespace, value)
	}
	return result, nil
}

func LookupMemoJSON[T any](ctx context.Context, runtime *Runtime, req MemoRequest) (T, bool) {
	var zero T
	if runtime == nil {
		return zero, false
	}
	key, err := memoIdentity(runtime, req)
	if err != nil {
		return zero, false
	}
	snapshot, ok := runtime.store.Load(ctx, key)
	if !ok {
		return zero, false
	}
	value, err := decodeMemo[T](snapshot)
	if err != nil {
		return zero, false
	}
	return value, true
}

func memoIdentity(runtime *Runtime, req MemoRequest) (string, error) {
	payload, err := json.Marshal(struct {
		Version   string `json:"version"`
		Namespace string `json:"namespace"`
		DataScope string `json:"dataScope"`
		Input     any    `json:"input"`
	}{runtime.version, req.Namespace, req.DataScope, req.Input})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return strings.TrimSpace(req.Namespace) + ":memo:" + fmt.Sprintf("%x", sum[:]), nil
}

func decodeMemo[T any](snapshot *ExecutionSnapshot) (T, error) {
	var zero T
	if snapshot == nil || snapshot.Datasets["memo"] == nil {
		return zero, fmt.Errorf("memo snapshot is empty")
	}
	fr := snapshot.Datasets["memo"].Primary()
	if fr == nil {
		return zero, fmt.Errorf("memo frame is empty")
	}
	field, ok := fr.Field("json")
	if !ok || len(field.Values) != 1 {
		return zero, fmt.Errorf("memo payload is invalid")
	}
	raw, ok := field.Values[0].(string)
	if !ok {
		return zero, fmt.Errorf("memo payload has type %T", field.Values[0])
	}
	var result T
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return zero, err
	}
	return result, nil
}
