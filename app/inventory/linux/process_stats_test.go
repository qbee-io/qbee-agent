package linux

import (
	"reflect"
	"testing"
)

func TestNewProcessStats(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		want    ProcessStats
		wantErr bool
	}{
		{
			name: "command with no spaces",
			line: "275809 (test-cmd) S 264817",
			want: ProcessStats{"275809", "test-cmd", "S", "264817"},
		},
		{
			name: "command with spaces",
			line: "275809 (test cmd) S 264817",
			want: ProcessStats{"275809", "test cmd", "S", "264817"},
		},
		{
			name:    "invalid format",
			line:    "275809 test cmd) S 264817",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewProcessStats(tt.line)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewProcessStats() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewProcessStats() got = %v, want %v", got, tt.want)
			}
		})
	}
}
