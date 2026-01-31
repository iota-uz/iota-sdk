package scalars

import (
	"fmt"
	"io"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/google/uuid"
)

// MarshalUUID marshals uuid.UUID to GraphQL
func MarshalUUID(id uuid.UUID) graphql.Marshaler {
	return graphql.WriterFunc(func(w io.Writer) {
		_, _ = io.WriteString(w, fmt.Sprintf(`"%s"`, id.String()))
	})
}

// UnmarshalUUID unmarshals GraphQL input to uuid.UUID
func UnmarshalUUID(v interface{}) (uuid.UUID, error) {
	str, ok := v.(string)
	if !ok {
		return uuid.UUID{}, fmt.Errorf("UUID must be a string")
	}
	return uuid.Parse(str)
}

// MarshalTime marshals time.Time to GraphQL (RFC3339)
func MarshalTime(t time.Time) graphql.Marshaler {
	return graphql.WriterFunc(func(w io.Writer) {
		_, _ = io.WriteString(w, fmt.Sprintf(`"%s"`, t.Format(time.RFC3339)))
	})
}

// UnmarshalTime unmarshals GraphQL input to time.Time
func UnmarshalTime(v interface{}) (time.Time, error) {
	str, ok := v.(string)
	if !ok {
		return time.Time{}, fmt.Errorf("Time must be a string")
	}
	return time.Parse(time.RFC3339, str)
}

// MarshalInt64 marshals int64 to GraphQL
func MarshalInt64(i int64) graphql.Marshaler {
	return graphql.WriterFunc(func(w io.Writer) {
		_, _ = io.WriteString(w, fmt.Sprintf(`"%d"`, i))
	})
}

// UnmarshalInt64 unmarshals GraphQL input to int64
func UnmarshalInt64(v interface{}) (int64, error) {
	switch v := v.(type) {
	case string:
		var i int64
		_, err := fmt.Sscanf(v, "%d", &i)
		return i, err
	case int:
		return int64(v), nil
	case int64:
		return v, nil
	case int32:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("Int64 must be a string or number")
	}
}

// MarshalJSON marshals map to GraphQL JSON
func MarshalJSON(v map[string]interface{}) graphql.Marshaler {
	return graphql.MarshalMap(v)
}

// UnmarshalJSON unmarshals GraphQL input to map
func UnmarshalJSON(v interface{}) (map[string]interface{}, error) {
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("JSON must be an object")
	}
	return m, nil
}
