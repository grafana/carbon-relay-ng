package whisper

import (
	"fmt"
	"sort"
	"testing"
)

func testParseRetentionDef(t *testing.T, retentionDef string, expectedPrecision, expectedPoints int, hasError bool) {
	errTpl := fmt.Sprintf("Expected %%v to be %%v but received %%v for retentionDef %v", retentionDef)

	retention, err := ParseRetentionDef(retentionDef)

	if (err == nil && hasError) || (err != nil && !hasError) {
		if hasError {
			t.Fatalf("Expected error but received none for retentionDef %v", retentionDef)
		} else {
			t.Fatalf("Expected no error but received %v for retentionDef %v", err, retentionDef)
		}
	}
	if err == nil {
		if retention.secondsPerPoint != expectedPrecision {
			t.Fatalf(errTpl, "precision", expectedPrecision, retention.secondsPerPoint)
		}
		if retention.numberOfPoints != expectedPoints {
			t.Fatalf(errTpl, "points", expectedPoints, retention.numberOfPoints)
		}
	}
}

func TestParseRetentionDef(t *testing.T) {
	testParseRetentionDef(t, "1s:5min", 1, 300, false)
	testParseRetentionDef(t, "1s:5minaf", 1, 300, true)
	testParseRetentionDef(t, "1m:30m", 60, 30, false)
	testParseRetentionDef(t, "1m", 0, 0, true)
	testParseRetentionDef(t, "1m:30m:20s", 0, 0, true)
	testParseRetentionDef(t, "1f:30seconds", 0, 0, true)
	testParseRetentionDef(t, "1m:30f", 0, 0, true)
}

func TestParseRetentionDefs(t *testing.T) {
	retentions, err := ParseRetentionDefs("1s:5m,1m:30m")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if length := len(retentions); length != 2 {
		t.Fatalf("Expected 2 retentions, received %v", length)
	}
}

func TestSortRetentions(t *testing.T) {
	retentions := Retentions{{300, 12}, {60, 30}, {1, 300}}
	sort.Sort(retentionsByPrecision{retentions})
	if retentions[0].secondsPerPoint != 1 {
		t.Fatalf("Retentions array is not sorted")
	}
}
