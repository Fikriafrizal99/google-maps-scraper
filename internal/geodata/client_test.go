package geodata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestProvincesFiltersJavaSumatraAndCaches(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path != "/provinces.json" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`[
			{"id":"11","name":"ACEH"},
			{"id":"32","name":"JAWA BARAT"},
			{"id":"51","name":"BALI"}
		]`))
	}))
	defer server.Close()

	client := NewWithBaseURL(t.TempDir(), server.URL)
	first, err := client.Provinces(context.Background())
	if err != nil {
		t.Fatalf("Provinces() error = %v", err)
	}
	if len(first) != 2 {
		t.Fatalf("Provinces() len = %d, want 2", len(first))
	}
	if first[0].ID != "11" || first[1].ID != "32" {
		t.Fatalf("unexpected provinces: %#v", first)
	}

	second, err := client.Provinces(context.Background())
	if err != nil {
		t.Fatalf("Provinces() second call error = %v", err)
	}
	if len(second) != 2 {
		t.Fatalf("Provinces() second len = %d, want 2", len(second))
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("upstream hits = %d, want 1 because second call should use cache", got)
	}
}

func TestRegenciesRejectsProvinceOutsideScope(t *testing.T) {
	t.Parallel()

	client := NewWithBaseURL(t.TempDir(), "http://127.0.0.1:1")
	if _, err := client.Regencies(context.Background(), "51"); err == nil {
		t.Fatal("Regencies() expected error for province outside Java-Sumatra")
	}
}

func TestDistrictsRejectsInvalidID(t *testing.T) {
	t.Parallel()

	client := NewWithBaseURL(t.TempDir(), "http://127.0.0.1:1")
	if _, err := client.Districts(context.Background(), "32/evil"); err == nil {
		t.Fatal("Districts() expected invalid id error")
	}
}
