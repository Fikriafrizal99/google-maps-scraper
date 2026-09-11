package collectorconfig

import (
	"path/filepath"
	"testing"
)

func TestJavaSumatraKostConfig(t *testing.T) {
	root := filepath.Join("..", "..")

	preset, err := LoadPreset(filepath.Join(root, "config", "presets", "kost.json"))
	if err != nil {
		t.Fatalf("LoadPreset() error = %v", err)
	}
	area, err := LoadArea(filepath.Join(root, "config", "areas", "java-sumatra.json"))
	if err != nil {
		t.Fatalf("LoadArea() error = %v", err)
	}

	if got, want := len(preset.Keywords), 9; got != want {
		t.Fatalf("keyword count = %d, want %d", got, want)
	}
	if got, want := len(area.Subareas), 16; got != want {
		t.Fatalf("subarea count = %d, want %d", got, want)
	}

	queries, err := BuildQueries(preset, area, "")
	if err != nil {
		t.Fatalf("BuildQueries() error = %v", err)
	}
	if got, want := len(queries), 144; got != want {
		t.Fatalf("query count = %d, want %d", got, want)
	}

	for _, name := range []string{"Jawa Barat", "Jawa Timur", "Aceh", "Sumatera Utara", "Lampung"} {
		provinceQueries, err := BuildQueries(preset, area, name)
		if err != nil {
			t.Fatalf("BuildQueries(%q) error = %v", name, err)
		}
		if got, want := len(provinceQueries), 9; got != want {
			t.Fatalf("query count for %s = %d, want %d", name, got, want)
		}
	}
}
