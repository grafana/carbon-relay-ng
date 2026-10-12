package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInvalidInstrumentationStartup(t *testing.T) {
	if configPath := os.Getenv("CARBON_RELAY_NG_TEST_INVALID_INSTRUMENTATION"); configPath != "" {
		os.Args = []string{os.Args[0], configPath}
		main()
		t.Fatal("startup accepted invalid instrumentation")
	}

	for _, interval := range []string{"", "graphite_interval = 0", "graphite_interval = -1000", "graphite_interval = 999"} {
		t.Run(interval, func(t *testing.T) {
			dir := t.TempDir()
			pidPath := filepath.Join(dir, "relay.pid")
			configPath := filepath.Join(dir, "relay.ini")
			configText := fmt.Sprintf("instance = 'test'\nlog_level = 'info'\nbad_metrics_max_age = '1h'\npid_file = %q\n[instrumentation]\ngraphite_addr = '127.0.0.1:1'\n%s\n", pidPath, interval)
			if err := os.WriteFile(configPath, []byte(configText), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestInvalidInstrumentationStartup$")
			cmd.Env = append(os.Environ(), "CARBON_RELAY_NG_TEST_INVALID_INSTRUMENTATION="+configPath)
			output, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatal("expected startup to fail")
			}
			if !strings.Contains(string(output), "instrumentation.graphite_interval must be at least 1000 ms") {
				t.Fatalf("expected actionable configuration error, got: %s", output)
			}
			if strings.Contains(string(output), "panic:") {
				t.Fatalf("configuration error caused a panic: %s", output)
			}
			if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
				t.Fatalf("startup created a pid file before validating instrumentation: %v", err)
			}
		})
	}
}
