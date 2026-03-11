package execution

import "testing"

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

			if got := controlDatabaseName(tt.operation); got != tt.want {
				t.Fatalf("controlDatabaseName(%q) = %q, want %q", tt.operation, got, tt.want)
			}
		})
	}
}
