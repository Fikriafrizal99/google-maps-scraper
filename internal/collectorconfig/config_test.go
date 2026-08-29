package collectorconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPreset(t *testing.T) {
	t.Parallel()

	path := writeTempConfig(t, `{
		"name":"kost",
		"keywords":["kos","kost"],
		"filters":{},
		"dedup":{"keys":["place_id"]},
		"output_fields":["title","address"]
	}`)

	preset, err := LoadPreset(path)
	if err != nil {
		t.Fatalf("LoadPreset() error = %v", err)
	}
	if preset.Name != "kost" {
		t.Fatalf("LoadPreset() name = %q, want %q", preset.Name, "kost")
	}
	if len(preset.Keywords) != 2 {
		t.Fatalf("LoadPreset() keywords = %d, want 2", len(preset.Keywords))
	}
}

func TestLoadArea(t *testing.T) {
	t.Parallel()

	path := writeTempConfig(t, `{
		"name":"jakarta",
		"display_name":"DKI Jakarta",
		"country":"Indonesia",
		"search_suffix":"Jakarta, Indonesia",
		"subareas":[{"name":"Jakarta Selatan","search_suffix":"Jakarta Selatan, DKI Jakarta"}]
	}`)

	area, err := LoadArea(path)
	if err != nil {
		t.Fatalf("LoadArea() error = %v", err)
	}
	if area.Name != "jakarta" {
		t.Fatalf("LoadArea() name = %q, want %q", area.Name, "jakarta")
	}
}

func TestPresetValidation(t *testing.T) {
	t.Parallel()

	preset := Preset{Name: "empty"}
	if err := preset.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want validation error")
	}
}

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}
