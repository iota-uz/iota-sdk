package billing

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatus_IsActive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{name: "created", status: Created, want: true},
		{name: "pending", status: Pending, want: true},
		{name: "completed", status: Completed, want: false},
		{name: "failed", status: Failed, want: false},
		{name: "canceled", status: Canceled, want: false},
		{name: "refunded", status: Refunded, want: false},
		{name: "partially refunded", status: PartiallyRefunded, want: false},
		{name: "expired", status: Expired, want: false},
		{name: "unknown", status: Status("unknown"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.status.IsActive())
		})
	}
}

func TestStatus_IsRefundable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{name: "created", status: Created, want: false},
		{name: "pending", status: Pending, want: false},
		{name: "completed", status: Completed, want: true},
		{name: "failed", status: Failed, want: false},
		{name: "canceled", status: Canceled, want: false},
		{name: "refunded", status: Refunded, want: false},
		{name: "partially refunded", status: PartiallyRefunded, want: true},
		{name: "expired", status: Expired, want: false},
		{name: "unknown", status: Status("unknown"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.status.IsRefundable())
		})
	}
}
