package collectorconfig

import "testing"

func TestBuildQueriesAllSubareas(t *testing.T) {
	t.Parallel()

	preset := Preset{Name: "kost", Keywords: []string{"kos", "kost"}, OutputFields: []string{"title"}}
	area := Area{
		Name:         "jakarta",
		SearchSuffix: "Jakarta, Indonesia",
		Subareas: []Subarea{
			{Name: "Jakarta Selatan", SearchSuffix: "Jakarta Selatan, DKI Jakarta"},
			{Name: "Jakarta Barat", SearchSuffix: "Jakarta Barat, DKI Jakarta"},
		},
	}

	queries, err := BuildQueries(preset, area, "")
	if err != nil {
		t.Fatalf("BuildQueries() error = %v", err)
	}
	if len(queries) != 4 {
		t.Fatalf("BuildQueries() len = %d, want 4", len(queries))
	}
}

func TestBuildQueriesSelectedSubarea(t *testing.T) {
	t.Parallel()

	preset := Preset{Name: "kost", Keywords: []string{"kos"}, OutputFields: []string{"title"}}
	area := Area{
		Name:         "jakarta",
		SearchSuffix: "Jakarta, Indonesia",
		Subareas: []Subarea{{Name: "Jakarta Selatan", SearchSuffix: "Jakarta Selatan, DKI Jakarta"}},
	}

	queries, err := BuildQueries(preset, area, "jakarta selatan")
	if err != nil {
		t.Fatalf("BuildQueries() error = %v", err)
	}
	if got, want := queries[0], "kos Jakarta Selatan, DKI Jakarta"; got != want {
		t.Fatalf("query = %q, want %q", got, want)
	}
}

func TestBuildQueriesForLocation(t *testing.T) {
	t.Parallel()

	preset := Preset{
		Name:         "b2b-prospecting",
		Keywords:     []string{"bengkel motor", "toko bangunan"},
		OutputFields: []string{"title"},
	}
	location := "SUKAMULYA, CUGENANG, KABUPATEN CIANJUR, JAWA BARAT, Indonesia"

	queries, err := BuildQueriesForLocation(preset, location)
	if err != nil {
		t.Fatalf("BuildQueriesForLocation() error = %v", err)
	}
	if len(queries) != 2 {
		t.Fatalf("BuildQueriesForLocation() len = %d, want 2", len(queries))
	}
	if got, want := queries[0], "bengkel motor "+location; got != want {
		t.Fatalf("query = %q, want %q", got, want)
	}
}
