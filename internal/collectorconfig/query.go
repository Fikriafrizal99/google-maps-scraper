package collectorconfig

import (
	"fmt"
	"strings"
)

// BuildQueries combines preset keywords with an area's search suffixes.
// When subarea is empty, all configured subareas are used; if the area has no
// subareas, the area's own search suffix is used.
func BuildQueries(preset Preset, area Area, subarea string) ([]string, error) {
	suffixes, err := resolveSuffixes(area, subarea)
	if err != nil {
		return nil, err
	}

	queries := make([]string, 0, len(preset.Keywords)*len(suffixes))
	seen := make(map[string]struct{}, cap(queries))

	for _, keyword := range preset.Keywords {
		keyword = strings.TrimSpace(keyword)
		if keyword == "" {
			continue
		}
		for _, suffix := range suffixes {
			query := strings.TrimSpace(keyword + " " + suffix)
			key := strings.ToLower(query)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			queries = append(queries, query)
		}
	}

	if len(queries) == 0 {
		return nil, fmt.Errorf("no queries generated")
	}

	return queries, nil
}

func resolveSuffixes(area Area, subarea string) ([]string, error) {
	if subarea != "" {
		for _, candidate := range area.Subareas {
			if strings.EqualFold(candidate.Name, subarea) {
				return []string{candidate.SearchSuffix}, nil
			}
		}
		return nil, fmt.Errorf("subarea %q not found in area %q", subarea, area.Name)
	}

	if len(area.Subareas) == 0 {
		return []string{area.SearchSuffix}, nil
	}

	suffixes := make([]string, 0, len(area.Subareas))
	for _, candidate := range area.Subareas {
		if strings.TrimSpace(candidate.SearchSuffix) != "" {
			suffixes = append(suffixes, candidate.SearchSuffix)
		}
	}
	if len(suffixes) == 0 {
		return []string{area.SearchSuffix}, nil
	}
	return suffixes, nil
}
