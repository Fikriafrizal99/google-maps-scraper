package main

import (
	"net/url"
	"reflect"
	"testing"
)

func TestSafeDashboardURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "empty", raw: "", want: "/"},
		{name: "root", raw: "/", want: "/"},
		{name: "filtered dashboard", raw: "/?q=melati&page=3&segment=putri", want: "/?q=melati&page=3&segment=putri"},
		{name: "reject absolute", raw: "https://example.com/?q=melati", want: "/"},
		{name: "reject other local path", raw: "/queue?q=melati", want: "/"},
	}

	for _, tt := range tests {
		t := tt
		testName := tt.name
		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			if got := safeDashboardURL(tt.raw); got != tt.want {
				t.Fatalf("safeDashboardURL(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestDetailURLPreservesDashboardContext(t *testing.T) {
	t.Parallel()

	got := detailURL(42, "/?page=3&q=kost&segment=putri")
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse detail URL: %v", err)
	}
	if parsed.Path != "/lead/42" {
		t.Fatalf("unexpected detail path %q", parsed.Path)
	}
	if gotFrom := parsed.Query().Get("from"); gotFrom != "/?page=3&q=kost&segment=putri" {
		t.Fatalf("unexpected from context %q", gotFrom)
	}
}

func TestDetailRedirectPreservesContextAndNotice(t *testing.T) {
	t.Parallel()

	got := detailRedirectURL(7, "/?page=2&verification_status=needs_check", "review")
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse redirect URL: %v", err)
	}
	if parsed.Path != "/lead/7" {
		t.Fatalf("unexpected redirect path %q", parsed.Path)
	}
	if gotFrom := parsed.Query().Get("from"); gotFrom != "/?page=2&verification_status=needs_check" {
		t.Fatalf("unexpected from context %q", gotFrom)
	}
	if gotNotice := parsed.Query().Get("review"); gotNotice != "saved" {
		t.Fatalf("unexpected review notice %q", gotNotice)
	}
}

func TestPaginationWindow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		current int
		total   int
		want    []int
	}{
		{name: "first page", current: 1, total: 10, want: []int{1, 2, 3, 4, 5}},
		{name: "middle page", current: 5, total: 10, want: []int{3, 4, 5, 6, 7}},
		{name: "last page", current: 10, total: 10, want: []int{6, 7, 8, 9, 10}},
		{name: "short result", current: 2, total: 3, want: []int{1, 2, 3}},
		{name: "empty result fallback", current: 1, total: 0, want: []int{1}},
	}

	for _, tt := range tests {
		t := tt
		testName := tt.name
		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			if got := paginationWindow(tt.current, tt.total); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("paginationWindow(%d, %d) = %v, want %v", tt.current, tt.total, got, tt.want)
			}
		})
	}
}
