package execution

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestControlDatabaseName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		operation string
		want      string
	}{
		{name: "e2e create uses postgres", operation: "db.e2e.create", want: "postgres"},
		{name: "e2e drop uses postgres", operation: "db.e2e.drop", want: "postgres"},
		{name: "e2e reset uses postgres", operation: "db.e2e.reset", want: "postgres"},
		{name: "seed main uses default database", operation: "seed.main", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, controlDatabaseName(tt.operation), "operation=%s", tt.operation)
		})
	}
}
