package utils

import (
	"testing"
)

func Test_sanitizeHostname(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		want     string
	}{
		{
			name:     "valid hostname",
			hostname: "valid-hostname",
			want:     "valid-hostname",
		},
		{
			name:     "hostname with invalid characters",
			hostname: "invalid!hostname@",
			want:     "invalidhostname",
		},
		{
			name:     "uppercase",
			hostname: "-UPPERCASE-HOSTNAME-",
			want:     "-UPPERCASE-HOSTNAME-",
		},
		{
			name:     "hostname with spaces",
			hostname: "host name with spaces",
			want:     "hostnamewithspaces",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeHostname(tt.hostname)
			if tt.want != got {
				t.Errorf("sanitizeHostname() = %v, want %v", got, tt.want)
			}
		})
	}
}
