package viewmodels

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHostOnly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		addr string
		want string
	}{
		{name: "ipv4 with port", addr: "203.0.113.7:51234", want: "203.0.113.7"},
		{name: "ipv6 with port", addr: "[2001:db8::1]:443", want: "2001:db8::1"},
		{name: "ipv4 without port", addr: "203.0.113.7", want: "203.0.113.7"},
		{name: "ipv6 without port", addr: "2001:db8::1", want: "2001:db8::1"},
		{name: "empty", addr: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, hostOnly(tt.addr))
		})
	}
}
