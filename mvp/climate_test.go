package main

import (
	"strings"
	"testing"
)

// v0.7.23 — Issue 27 tests. Catalog lookup, validation, and shape.

func TestLookupClimateMode_KnownAndUnknown(t *testing.T) {
	for _, code := range []string{"normal", "heat_burst_anthesis", "heat_grain_filling", "combined_postflowering", "drought_terminal", "salinity_chronic", "heat_burst_booting", "prolonged_heat"} {
		if _, err := LookupClimateMode(code); err != nil {
			t.Errorf("known mode %q should not error: %v", code, err)
		}
	}
	if _, err := LookupClimateMode("frankenmode"); err == nil {
		t.Error("unknown mode should error")
	}
}

func TestValidateClimateScenario_RejectsNegativeSeverity(t *testing.T) {
	s := &ClimateScenario{Mode: "heat_burst_anthesis", Severity: -0.1}
	if err := ValidateClimateScenario(s); err == nil {
		t.Error("negative severity should be rejected")
	}
}

func TestValidateClimateScenario_RejectsUnknownMode(t *testing.T) {
	s := &ClimateScenario{Mode: "tornado", Severity: 1.0}
	err := ValidateClimateScenario(s)
	if err == nil || !strings.Contains(err.Error(), "unknown climate mode") {
		t.Errorf("unknown mode should fail with descriptive error; got %v", err)
	}
}

func TestValidateClimateScenario_AcceptsZeroSeverity(t *testing.T) {
	s := &ClimateScenario{Mode: "heat_burst_anthesis", Severity: 0}
	if err := ValidateClimateScenario(s); err != nil {
		t.Errorf("zero severity should be allowed, got %v", err)
	}
}

func TestClimateModesCatalog_StableAlphabeticalOrder(t *testing.T) {
	cat := ClimateModesCatalog()
	if len(cat) < 8 {
		t.Errorf("expected ≥ 8 modes in catalog, got %d", len(cat))
	}
	for i := 1; i < len(cat); i++ {
		if cat[i-1].Code > cat[i].Code {
			t.Errorf("catalog not sorted: %q > %q", cat[i-1].Code, cat[i].Code)
		}
	}
}

func TestClimateModesCatalog_AnthesisWindowsAreCorrect(t *testing.T) {
	// Sanity: anthesis-burst window is [-3, +3]; booting is [-18, -10];
	// grain-fill is [+5, +25]. These are the canonical Khan-et-al-2024 +
	// Porter-Gawith-1999 windows the catalog encodes.
	m, _ := LookupClimateMode("heat_burst_anthesis")
	if m.WindowStart != -3 || m.WindowEnd != 3 {
		t.Errorf("anthesis window expected [-3, +3], got [%d, %d]", m.WindowStart, m.WindowEnd)
	}
	m, _ = LookupClimateMode("heat_burst_booting")
	if m.WindowStart != -18 || m.WindowEnd != -10 {
		t.Errorf("booting window expected [-18, -10], got [%d, %d]", m.WindowStart, m.WindowEnd)
	}
	m, _ = LookupClimateMode("heat_grain_filling")
	if m.WindowStart != 5 || m.WindowEnd != 25 {
		t.Errorf("grain-filling window expected [+5, +25], got [%d, %d]", m.WindowStart, m.WindowEnd)
	}
}
