package collectorpost

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/gosom/google-maps-scraper/internal/collectorconfig"
)

// ProcessCSV filters, deduplicates, and projects raw scraper CSV according to a preset.
func ProcessCSV(inputPath, outputPath string, preset collectorconfig.Preset) error {
	in, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("open raw csv: %w", err)
	}
	defer in.Close()

	reader := csv.NewReader(in)
	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("read csv header: %w", err)
	}
	index := headerIndex(header)

	fields := normalizeOutputFields(preset.OutputFields)
	for _, field := range fields {
		if _, ok := index[field]; !ok {
			return fmt.Errorf("output field %q not found in scraper CSV", field)
		}
	}

	out, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output csv: %w", err)
	}
	defer out.Close()

	writer := csv.NewWriter(out)
	defer writer.Flush()

	if err := writer.Write(fields); err != nil {
		return fmt.Errorf("write output header: %w", err)
	}

	seen := make(map[string]struct{})
	for {
		row, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("read csv row: %w", err)
		}

		if !matchesFilters(row, index, preset.Filters) {
			continue
		}

		key := dedupKey(row, index, preset.Dedup.Keys)
		if key != "" {
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
		}

		projected := make([]string, len(fields))
		for i, field := range fields {
			projected[i] = value(row, index, field)
		}
		if err := writer.Write(projected); err != nil {
			return fmt.Errorf("write output row: %w", err)
		}
	}

	writer.Flush()
	return writer.Error()
}

func headerIndex(header []string) map[string]int {
	index := make(map[string]int, len(header))
	for i, field := range header {
		index[strings.ToLower(strings.TrimSpace(field))] = i
	}
	return index
}

func normalizeOutputFields(fields []string) []string {
	aliases := map[string]string{
		"website":   "website",
		"rating":    "review_rating",
		"reviews":   "review_count",
		"longitude": "longitude",
	}
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.ToLower(strings.TrimSpace(field))
		if mapped, ok := aliases[field]; ok {
			field = mapped
		}
		out = append(out, field)
	}
	return out
}

func matchesFilters(row []string, index map[string]int, cfg collectorconfig.FilterConfig) bool {
	for _, field := range cfg.RequiredFields {
		if strings.TrimSpace(value(row, index, normalizeField(field))) == "" {
			return false
		}
	}
	if cfg.HasPhone && strings.TrimSpace(value(row, index, "phone")) == "" {
		return false
	}
	if cfg.HasWebsite && strings.TrimSpace(value(row, index, "website")) == "" {
		return false
	}
	if cfg.MinRating > 0 {
		rating, _ := strconv.ParseFloat(value(row, index, "review_rating"), 64)
		if rating < cfg.MinRating {
			return false
		}
	}

	title := strings.ToLower(value(row, index, "title"))
	if len(cfg.IncludeTitlePatterns) > 0 && !containsAny(title, cfg.IncludeTitlePatterns) {
		return false
	}
	if containsAny(title, cfg.ExcludeTitlePatterns) {
		return false
	}
	return true
}

func containsAny(haystack string, patterns []string) bool {
	for _, pattern := range patterns {
		pattern = strings.ToLower(strings.TrimSpace(pattern))
		if pattern != "" && strings.Contains(haystack, pattern) {
			return true
		}
	}
	return false
}

func dedupKey(row []string, index map[string]int, keys []string) string {
	for _, key := range keys {
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "place_id":
			if v := normalized(value(row, index, "place_id")); v != "" {
				return "place_id:" + v
			}
		case "data_id":
			if v := normalized(value(row, index, "data_id")); v != "" {
				return "data_id:" + v
			}
		case "phone":
			if v := normalizedPhone(value(row, index, "phone")); v != "" {
				return "phone:" + v
			}
		case "title+coordinates":
			title := normalized(value(row, index, "title"))
			lat := normalized(value(row, index, "latitude"))
			lon := normalized(value(row, index, "longitude"))
			if title != "" && lat != "" && lon != "" {
				return "geo:" + title + "|" + lat + "|" + lon
			}
		}
	}
	return ""
}

func normalizeField(field string) string {
	switch strings.ToLower(strings.TrimSpace(field)) {
	case "rating":
		return "review_rating"
	case "reviews":
		return "review_count"
	default:
		return strings.ToLower(strings.TrimSpace(field))
	}
}

func normalized(v string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(v)), " "))
}

func normalizedPhone(v string) string {
	var b strings.Builder
	for _, r := range v {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func value(row []string, index map[string]int, field string) string {
	i, ok := index[field]
	if !ok || i < 0 || i >= len(row) {
		return ""
	}
	return row[i]
}
