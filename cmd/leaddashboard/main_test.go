package main

import "testing"

func TestWANumber(t *testing.T) {
	t.Parallel()
	if got := waNumber("0812-3456-7890"); got != "6281234567890" {
		t.Fatalf("unexpected wa number %q", got)
	}
	if got := waNumber("+62 812 3456 7890"); got != "6281234567890" {
		t.Fatalf("unexpected international wa number %q", got)
	}
}

func TestSafeConfigName(t *testing.T) {
	t.Parallel()
	if !safeConfigName("jakarta-selatan_1") {
		t.Fatal("expected safe config name")
	}
	if safeConfigName("../jakarta") {
		t.Fatal("path traversal must be rejected")
	}
}

func TestParseBoundedInt(t *testing.T) {
	t.Parallel()
	if got := parseBoundedInt("99", 5, 1, 30); got != 30 {
		t.Fatalf("expected max clamp, got %d", got)
	}
	if got := parseBoundedInt("bad", 5, 1, 30); got != 5 {
		t.Fatalf("expected fallback, got %d", got)
	}
}
