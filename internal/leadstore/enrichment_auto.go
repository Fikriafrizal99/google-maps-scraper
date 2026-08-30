package leadstore

import (
	"context"
	"strings"
)

type EnrichmentSummary struct {
	Total      int `json:"total"`
	Detected   int `json:"detected"`
	NoSignal   int `json:"no_signal"`
	Applied    int `json:"applied"`
	ManualKept int `json:"manual_kept"`
}

func EvaluateKostEnrichment(lead Lead) KostEnrichment {
	e := defaultKostEnrichment(lead.ID)
	text := normalizeText(strings.Join([]string{lead.Title, lead.Category, lead.Address}, " "))
	title := normalizeText(lead.Title)

	switch {
	case containsAnyPhrase(text, "suami istri", "pasutri", "pasangan suami istri"):
		e.Segment = "pasutri"
	case containsAnyWord(text, "campur", "campuran") || containsAnyPhrase(text, "putra putri", "putri putra"):
		e.Segment = "campur"
	case containsAnyWord(text, "putri", "wanita", "perempuan"):
		e.Segment = "putri"
	case containsAnyWord(text, "putra", "pria", "laki") || containsAnyPhrase(text, "laki laki"):
		e.Segment = "putra"
	}

	switch {
	case containsAnyWord(text, "mahasiswa", "mahasiswi") || containsAnyPhrase(text, "anak kuliah"):
		e.Target = "mahasiswa"
	case containsAnyWord(text, "karyawan", "karyawati", "pekerja"):
		e.Target = "karyawan"
	case containsAnyWord(text, "keluarga"):
		e.Target = "keluarga"
	}

	rental := make([]string, 0, 4)
	if containsAnyWord(text, "harian", "daily") {
		rental = append(rental, "harian")
	}
	if containsAnyWord(text, "mingguan", "weekly") {
		rental = append(rental, "mingguan")
	}
	if containsAnyWord(text, "bulanan", "monthly") {
		rental = append(rental, "bulanan")
	}
	if containsAnyWord(text, "tahunan", "yearly") {
		rental = append(rental, "tahunan")
	}
	if len(rental) > 0 {
		e.RentalType = strings.Join(rental, ", ")
	}

	facilities := make([]string, 0, 8)
	if containsAnyPhrase(text, "wifi", "wi fi") {
		facilities = append(facilities, "WiFi")
	}
	if containsAnyWord(text, "ac") || containsAnyPhrase(text, "air conditioner", "air conditioning") {
		facilities = append(facilities, "AC")
	}
	if containsAnyPhrase(text, "kamar mandi dalam", "km dalam") {
		facilities = append(facilities, "Kamar mandi dalam")
	}
	if containsAnyWord(text, "parkir", "parking") {
		facilities = append(facilities, "Parkir")
	}
	if containsAnyWord(text, "dapur", "kitchen") {
		facilities = append(facilities, "Dapur")
	}
	if containsAnyWord(text, "laundry") {
		facilities = append(facilities, "Laundry")
	}
	if len(facilities) > 0 {
		e.Facilities = strings.Join(uniqueStrings(facilities), ", ")
	}

	switch {
	case containsAnyPhrase(text, "full furnished", "fully furnished"):
		e.Furnish = "full furnished"
	case containsAnyPhrase(text, "semi furnished"):
		e.Furnish = "semi furnished"
	case containsAnyWord(text, "furnished"):
		e.Furnish = "furnished"
	}

	rules := make([]string, 0, 4)
	if containsAnyPhrase(text, "boleh pasutri", "bisa pasutri") {
		rules = append(rules, "boleh pasutri")
	}
	if containsAnyPhrase(text, "pet friendly", "boleh hewan", "boleh peliharaan") {
		rules = append(rules, "pet friendly")
	}
	if containsAnyPhrase(text, "parkir mobil", "car parking") {
		rules = append(rules, "parkir mobil")
	}
	if len(rules) > 0 {
		e.Rules = strings.Join(uniqueStrings(rules), ", ")
	}

	// Selling point is intentionally conservative and only uses explicit title wording.
	selling := make([]string, 0, 4)
	if containsAnyWord(title, "exclusive", "eksklusif") {
		selling = append(selling, "Eksklusif")
	}
	if containsAnyWord(title, "premium") {
		selling = append(selling, "Premium")
	}
	if containsAnyPhrase(title, "pet friendly") {
		selling = append(selling, "Pet friendly")
	}
	if len(selling) > 0 {
		e.SellingPoint = strings.Join(uniqueStrings(selling), ", ")
	}

	return e
}

func HasEnrichmentSignal(e KostEnrichment) bool {
	return e.Segment != EnrichmentUnknown ||
		e.Target != EnrichmentUnknown ||
		e.RentalType != EnrichmentUnknown ||
		e.PriceRange != EnrichmentUnknown ||
		e.Facilities != EnrichmentUnknown ||
		e.Furnish != EnrichmentUnknown ||
		e.Rules != EnrichmentUnknown ||
		e.Landmark != EnrichmentUnknown ||
		e.SellingPoint != EnrichmentUnknown
}

func (s *Store) RunAutoEnrichment(ctx context.Context, filter Filter, apply bool) (EnrichmentSummary, error) {
	if filter.Limit <= 0 || filter.Limit > 5000 {
		filter.Limit = 5000
	}
	leads, err := s.List(ctx, filter)
	if err != nil {
		return EnrichmentSummary{}, err
	}
	if err := s.EnsureEnrichmentSchema(ctx); err != nil {
		return EnrichmentSummary{}, err
	}

	summary := EnrichmentSummary{Total: len(leads)}
	for _, lead := range leads {
		e := EvaluateKostEnrichment(lead)
		if HasEnrichmentSignal(e) {
			summary.Detected++
		} else {
			summary.NoSignal++
		}
		if !apply {
			continue
		}

		existing, err := s.GetEnrichment(ctx, lead.ID)
		if err != nil {
			return summary, err
		}
		if existing.Source == EnrichmentManual {
			summary.ManualKept++
			continue
		}
		if err := s.updateAutoEnrichment(ctx, e); err != nil {
			return summary, err
		}
		summary.Applied++
	}
	return summary, nil
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		key := strings.ToLower(strings.TrimSpace(value))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	return result
}
