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
		_, _ = w.Write([]byte(`{
			"data": [
				{"id":"11","name":"Aceh"},
				{"id":"32","name":"Jawa Barat"},
				{"id":"51","name":"Bali"}
			],
			"meta":{"level":1}
		}`))
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

func TestDecodeRegionsAcceptsLegacyBareArray(t *testing.T) {
	t.Parallel()

	regions, err := decodeRegions([]byte(`[{"id":"32","name":"JAWA BARAT"}]`))
	if err != nil {
		t.Fatalf("decodeRegions() legacy error = %v", err)
	}
	if len(regions) != 1 || regions[0].ID != "32" {
		t.Fatalf("unexpected legacy regions: %#v", regions)
	}
}

func TestRegenciesRejectsProvinceOutsideScope(t *testing.T) {
	t.Parallel()

	client := NewWithBaseURL(t.TempDir(), "http://127.0.0.1:1")
	if _, err := client.Regencies(context.Background(), "51"); err == nil {
		t.Fatal("Regencies() expected error for province outside Java-Sumatra")
	}
}

func TestDistrictsAcceptsV2DottedID(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/districts/32.01.json" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"32.01.01","name":"Bogor Selatan"}],"meta":{"level":3}}`))
	}))
	defer server.Close()

	client := NewWithBaseURL(t.TempDir(), server.URL)
	regions, err := client.Districts(context.Background(), "32.01")
	if err != nil {
		t.Fatalf("Districts() error = %v", err)
	}
	if len(regions) != 1 || regions[0].ID != "32.01.01" {
		t.Fatalf("unexpected districts: %#v", regions)
	}
}

func TestVillagesAcceptsV2DottedID(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/villages/32.01.01.json" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"32.01.01.1001","name":"Batu Tulis"}],"meta":{"level":4}}`))
	}))
	defer server.Close()

	client := NewWithBaseURL(t.TempDir(), server.URL)
	regions, err := client.Villages(context.Background(), "32.01.01")
	if err != nil {
		t.Fatalf("Villages() error = %v", err)
	}
	if len(regions) != 1 || regions[0].ID != "32.01.01.1001" {
		t.Fatalf("unexpected villages: %#v", regions)
	}
}

func TestDistrictsRejectsInvalidID(t *testing.T) {
	t.Parallel()

	client := NewWithBaseURL(t.TempDir(), "http://127.0.0.1:1")
	if _, err := client.Districts(context.Background(), "32/evil"); err == nil {
		t.Fatal("Districts() expected invalid id error")
	}
}
