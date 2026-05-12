package main

// TEST-FIX-3-COV smoke test for the haystack-derived coverage tool.
//
// Don't actually run `go test` (that would recurse infinitely). Just verify
// parseConfig honors the environment variable contract (THRESHOLD_TOTAL,
// BUILD_TAGS, and related variables).

import (
	"os"
	"testing"
)

func TestParseConfig_DefaultsAndEnvOverrides(t *testing.T) {
	// Save then restore the environment to avoid affecting other tests.
	saved := map[string]string{}
	for _, k := range []string{
		"CI", "THRESHOLD_FUNC", "THRESHOLD_PACKAGE", "THRESHOLD_PRINT",
		"THRESHOLD_TOTAL", "UNCOVERED_LIMIT", "EXCLUDE_FUNCS", "BUILD_TAGS",
		"COVERPROFILE",
	} {
		saved[k] = os.Getenv(k)
		os.Unsetenv(k)
	}
	t.Cleanup(func() {
		for k, v := range saved {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	})

	// Default (CI not set)
	c := parseConfig()
	if c.ThresholdTotal != 90.0 {
		t.Errorf("default ThresholdTotal=90, got %v", c.ThresholdTotal)
	}
	if c.CIMode {
		t.Error("CIMode should be false by default")
	}
	if c.RaceDetection {
		t.Error("RaceDetection should be false by default")
	}
	if c.BuildTags != "" {
		t.Errorf("BuildTags should be empty by default, got %q", c.BuildTags)
	}

	// CI mode + env overrides
	os.Setenv("CI", "true")
	os.Setenv("THRESHOLD_TOTAL", "85")
	os.Setenv("THRESHOLD_FUNC", "80")
	os.Setenv("BUILD_TAGS", "sqlite_fts5 race_heavy")
	os.Setenv("COVERPROFILE", "coverage.out")

	c = parseConfig()
	if !c.CIMode {
		t.Error("CIMode should be true when CI=true")
	}
	if !c.RaceDetection {
		t.Error("RaceDetection should be true in CI mode")
	}
	if c.ThresholdTotal != 85.0 {
		t.Errorf("ThresholdTotal env override failed: got %v want 85", c.ThresholdTotal)
	}
	if c.ThresholdFunc != 80.0 {
		t.Errorf("ThresholdFunc env override failed: got %v want 80", c.ThresholdFunc)
	}
	if c.BuildTags != "sqlite_fts5 race_heavy" {
		t.Errorf("BuildTags env override failed: got %q", c.BuildTags)
	}
	if c.CoverProfile != "coverage.out" {
		t.Errorf("CoverProfile env override failed: got %q", c.CoverProfile)
	}
}

func TestModulePrefix_BorgeeServer(t *testing.T) {
	// Sanity check: ensure the haystack-to-borgee port renamed ModulePrefix.
	if ModulePrefix != "borgee-server/" {
		t.Errorf("ModulePrefix should be %q, got %q", "borgee-server/", ModulePrefix)
	}
}
