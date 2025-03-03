package internet_test

import (
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"testing"
)

func TestNewIP(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		version     internet.IpVersion
		wantErr     bool
		wantValue   string
		wantVersion internet.IpVersion
	}{
		{
			name:        "valid IPv4",
			value:       "192.168.1.1",
			version:     internet.IPv4,
			wantErr:     false,
			wantValue:   "192.168.1.1",
			wantVersion: internet.IPv4,
		},
		{
			name:        "valid IPv6",
			value:       "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
			version:     internet.IPv6,
			wantErr:     false,
			wantValue:   "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
			wantVersion: internet.IPv6,
		},
		{
			name:    "invalid IPv4",
			value:   "256.256.256.256",
			version: internet.IPv4,
			wantErr: true,
		},
		{
			name:    "invalid IPv6",
			value:   "not:valid:ipv6:address",
			version: internet.IPv6,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip, err := internet.NewIP(tt.value, tt.version)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if got := ip.Value(); got != tt.wantValue {
				t.Errorf("IP.Value() = %v, want %v", got, tt.wantValue)
			}

			if got := ip.Version(); got != tt.wantVersion {
				t.Errorf("IP.Version() = %v, want %v", got, tt.wantVersion)
			}
		})
	}
}

func TestIsValidIP(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		version internet.IpVersion
		want    bool
	}{
		// IPv4 tests
		{
			name:    "valid IPv4",
			value:   "192.168.1.1",
			version: internet.IPv4,
			want:    true,
		},
		{
			name:    "invalid IPv4 - letters",
			value:   "abc.def.ghi.jkl",
			version: internet.IPv4,
			want:    false,
		},
		{
			name:    "invalid IPv4 - wrong format",
			value:   "192.168.1",
			version: internet.IPv4,
			want:    false,
		},
		{
			name:    "invalid IPv4 - extra numbers",
			value:   "192.168.1.1.1",
			version: internet.IPv4,
			want:    false,
		},

		// IPv6 tests
		{
			name:    "valid IPv6",
			value:   "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
			version: internet.IPv6,
			want:    true,
		},
		{
			name:    "invalid IPv6 - wrong format",
			value:   "2001:0db8:85a3:0000:0000:8a2e:0370",
			version: internet.IPv6,
			want:    false,
		},
		{
			name:    "invalid IPv6 - invalid characters",
			value:   "2001:0db8:85a3:0000:0000:8a2e:0370:xxxx",
			version: internet.IPv6,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := internet.IsValidIP(tt.value, tt.version); got != tt.want {
				t.Errorf("IsValidIP(%v, %v) = %v, want %v",
					tt.value, tt.version, got, tt.want)
			}
		})
	}
}

// Add more edge cases if needed:
func TestIPEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		version internet.IpVersion
		wantErr bool
	}{
		{
			name:    "empty string IPv4",
			value:   "",
			version: internet.IPv4,
			wantErr: true,
		},
		{
			name:    "empty string IPv6",
			value:   "",
			version: internet.IPv6,
			wantErr: true,
		},
		{
			name:    "IPv4 with spaces",
			value:   "192. 168. 1. 1",
			version: internet.IPv4,
			wantErr: true,
		},
		{
			name:    "IPv6 with spaces",
			value:   "2001:0db8: 85a3:0000:0000:8a2e:0370:7334",
			version: internet.IPv6,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := internet.NewIP(tt.value, tt.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewIP() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
