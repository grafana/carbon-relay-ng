package cfg

import (
	"fmt"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestValidateInstrumentation(t *testing.T) {
	for _, tc := range []struct {
		name     string
		addr     string
		interval string
		wantErr  bool
	}{
		{"omitted", "localhost:2003", "", true},
		{"zero", "localhost:2003", "graphite_interval = 0", true},
		{"negative", "localhost:2003", "graphite_interval = -1000", true},
		{"subsecond", "localhost:2003", "graphite_interval = 999", true},
		{"one second", "localhost:2003", "graphite_interval = 1000", false},
		{"fractional second", "localhost:2003", "graphite_interval = 1500", false},
		{"ten seconds", "localhost:2003", "graphite_interval = 10000", false},
		{"disabled omitted", "", "", false},
		{"disabled zero", "", "graphite_interval = 0", false},
		{"disabled negative", "", "graphite_interval = -1000", false},
		{"disabled subsecond", "", "graphite_interval = 999", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := NewConfig()
			text := fmt.Sprintf("[instrumentation]\ngraphite_addr = %q\n%s\n", tc.addr, tc.interval)
			if _, err := toml.Decode(text, &config); err != nil {
				t.Fatal(err)
			}
			if err := config.ValidateInstrumentation(); (err != nil) != tc.wantErr {
				t.Fatalf("ValidateInstrumentation() = %v, want error: %v", err, tc.wantErr)
			}
		})
	}
}
