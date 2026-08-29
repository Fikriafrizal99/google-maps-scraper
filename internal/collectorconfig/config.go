package collectorconfig

import (
	"encoding/json"
	"fmt"
	"os"
)

// Preset describes reusable search, filter, deduplication, and output rules.
type Preset struct {
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	Keywords     []string     `json:"keywords"`
	Filters      FilterConfig `json:"filters"`
	Dedup        DedupConfig  `json:"dedup"`
	OutputFields []string     `json:"output_fields"`
}

// FilterConfig contains post-scrape filtering rules.
type FilterConfig struct {
	RequiredFields       []string `json:"required_fields"`
	MinRating            float64  `json:"min_rating"`
	HasPhone             bool     `json:"has_phone"`
	HasWebsite           bool     `json:"has_website"`
	IncludeTitlePatterns []string `json:"include_title_patterns"`
	ExcludeTitlePatterns []string `json:"exclude_title_patterns"`
}

// DedupConfig defines key priority for duplicate detection.
type DedupConfig struct {
	Keys []string `json:"keys"`
}

// Area describes a geographic search scope and optional subdivisions.
type Area struct {
	Name         string    `json:"name"`
	DisplayName  string    `json:"display_name"`
	Country      string    `json:"country"`
	SearchSuffix string    `json:"search_suffix"`
	Subareas     []Subarea `json:"subareas"`
}

// Subarea describes one child search scope.
type Subarea struct {
	Name         string `json:"name"`
	SearchSuffix string `json:"search_suffix"`
}

// LoadPreset loads and validates a preset JSON file.
func LoadPreset(path string) (Preset, error) {
	var preset Preset
	if err := loadJSON(path, &preset); err != nil {
		return Preset{}, err
	}
	if err := preset.Validate(); err != nil {
		return Preset{}, fmt.Errorf("invalid preset %q: %w", path, err)
	}
	return preset, nil
}

// LoadArea loads and validates an area JSON file.
func LoadArea(path string) (Area, error) {
	var area Area
	if err := loadJSON(path, &area); err != nil {
		return Area{}, err
	}
	if err := area.Validate(); err != nil {
		return Area{}, fmt.Errorf("invalid area %q: %w", path, err)
	}
	return area, nil
}

// Validate checks the minimum required preset fields.
func (p Preset) Validate() error {
	if p.Name == "" {
		return fmt.Errorf("name is required")
	}
	if len(p.Keywords) == 0 {
		return fmt.Errorf("at least one keyword is required")
	}
	if len(p.OutputFields) == 0 {
		return fmt.Errorf("at least one output field is required")
	}
	return nil
}

// Validate checks the minimum required area fields.
func (a Area) Validate() error {
	if a.Name == "" {
		return fmt.Errorf("name is required")
	}
	if a.SearchSuffix == "" {
		return fmt.Errorf("search_suffix is required")
	}
	return nil
}

func loadJSON(path string, dst any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("decode config: %w", err)
	}
	return nil
}
