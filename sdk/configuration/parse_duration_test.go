package configuration

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	t.Parallel()
	type args struct {
		d string
	}
	tests := []struct {
		name    string
		args    args
		want    time.Duration
		wantErr bool
	}{
		{
			name: "Test 1",
			args: args{
				d: "1s",
			},
			want:    time.Second,
			wantErr: false,
		},
		{
			name: "Test 2",
			args: args{
				d: "1m",
			},
			want:    time.Minute,
			wantErr: false,
		},
		{
			name: "Test 3",
			args: args{
				d: "1h",
			},
			want:    time.Hour,
			wantErr: false,
		},
		{
			name: "Test 4",
			args: args{
				d: "1d",
			},
			want:    time.Hour * 24,
			wantErr: false,
		},
		{
			name: "Test 5",
			args: args{
				d: "1w",
			},
			want:    time.Hour * 24 * 7,
			wantErr: false,
		},
		{
			name: "Test 6",
			args: args{
				d: "1M",
			},
			want:    time.Hour * 24 * 30,
			wantErr: false,
		},
		{
			name: "Test 7",
			args: args{
				d: "1y",
			},
			want:    time.Hour * 24 * 365,
			wantErr: false,
		},
		{
			name: "Test 8",
			args: args{
				d: "1z",
			},
			want:    0,
			wantErr: true,
		},
		{
			name: "Test 9",
			args: args{
				d: "20m30",
			},
			want:    0,
			wantErr: true,
		},
		{
			name: "Test 10",
			args: args{
				d: "20m30s",
			},
			want:    20*time.Minute + 30*time.Second,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseDuration(tt.args.d)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDuration() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}
