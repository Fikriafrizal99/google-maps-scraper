package main

import "testing"

func TestApplyCollectProgressUsesLatestMarker(t *testing.T) {
	t.Parallel()

	state := collectState{Running: true, Stage: "running"}
	logText := "KOST_PROGRESS stage=starting queries=6 raw_rows=0 final_rows=0 imported_rows=0\n" +
		"engine output\n" +
		"KOST_PROGRESS stage=scraping queries=6 raw_rows=27 final_rows=0 imported_rows=0\n"

	applyCollectProgress(&state, logText)
	if state.Stage != "scraping" {
		t.Fatalf("stage = %q, want scraping", state.Stage)
	}
	if state.QueryCount != 6 || state.RawRows != 27 || state.FinalRows != 0 || state.ImportedRows != 0 {
		t.Fatalf("unexpected progress: %+v", state)
	}
}

func TestApplyCollectProgressPreservesTerminalDashboardState(t *testing.T) {
	t.Parallel()

	state := collectState{Running: false, Stage: "failed"}
	applyCollectProgress(&state, "KOST_PROGRESS stage=scraping queries=4 raw_rows=11 final_rows=0 imported_rows=0")
	if state.Stage != "failed" {
		t.Fatalf("terminal stage changed to %q", state.Stage)
	}
	if state.RawRows != 11 || state.QueryCount != 4 {
		t.Fatalf("counts were not retained: %+v", state)
	}
}

func TestCollectElapsed(t *testing.T) {
	t.Parallel()

	got := collectElapsed("2026-09-11 10:00:00", "2026-09-11 10:02:05", false)
	if got != "2m 05s" {
		t.Fatalf("collectElapsed() = %q, want 2m 05s", got)
	}
}
