package leadstore

import (
	"context"
	"fmt"
	"strings"
)

type QCEvaluation struct {
	Status  string   `json:"status"`
	Score   int      `json:"score"`
	Reasons []string `json:"reasons"`
}

type QCSummary struct {
	Total       int `json:"total"`
	Valid       int `json:"valid"`
	NeedsReview int `json:"needs_review"`
	Exclude     int `json:"exclude"`
	Applied     int `json:"applied"`
	ManualKept  int `json:"manual_kept"`
}

func EvaluateLeadQC(lead Lead) QCEvaluation {
	if strings.EqualFold(strings.TrimSpace(lead.Preset), "kost") || strings.TrimSpace(lead.Preset) == "" {
		return evaluateKostQC(lead)
	}
	return evaluateGenericQC(lead)
}

func evaluateKostQC(lead Lead) QCEvaluation {
	title := normalizeText(lead.Title)
	category := normalizeText(lead.Category)
	score := 45
	reasons := make([]string, 0, 8)

	strongKost := containsAnyPhrase(title,
		"rumah kost", "rumah kos", "kost exclusive", "kost eksklusif",
		"kos exclusive", "kos eksklusif", "indekos", "boarding house",
	)
	hasKost := strongKost || containsAnyWord(title, "kost", "kos", "boarding")

	if strongKost {
		score += 38
		reasons = append(reasons, "nama sangat kuat menunjukkan usaha kost")
	} else if hasKost {
		score += 30
		reasons = append(reasons, "nama mengandung sinyal kost")
	}

	if containsAnyWord(title, "putra", "putri") && hasKost {
		score += 5
		reasons = append(reasons, "nama menyebut segmentasi kost")
	}

	if strings.TrimSpace(lead.Address) != "" {
		score += 6
		reasons = append(reasons, "alamat tersedia")
	} else {
		score -= 8
		reasons = append(reasons, "alamat kosong")
	}
	if strings.TrimSpace(lead.Phone) != "" {
		score += 5
		reasons = append(reasons, "nomor telepon tersedia")
	}
	if strings.TrimSpace(lead.Website) != "" {
		score += 2
	}
	if lead.Rating >= 4.0 && lead.ReviewCount >= 5 {
		score += 3
	}

	// Google Maps categories are noisy for kost. Category is only a small signal.
	if containsAnyPhrase(category, "boarding house", "student dormitory") {
		score += 6
		reasons = append(reasons, "kategori mendukung kost")
	} else if containsAnyPhrase(category, "motel", "homestay", "guest house", "lodging", "inn") {
		reasons = append(reasons, "kategori ambigu; tidak dijadikan veto")
	}

	negativeTitle := containsAnyWord(title, "hotel", "hostel", "apartemen", "apartment", "resort", "villa", "oyo", "reddoorz")
	if negativeTitle && !hasKost {
		score -= 32
		reasons = append(reasons, "nama lebih kuat menunjukkan akomodasi non-kost")
	} else if negativeTitle && hasKost {
		reasons = append(reasons, "ada istilah non-kost, tetapi nama juga jelas menyebut kost")
	}

	if !hasKost && containsAnyPhrase(title, "guest house", "guesthouse", "homestay", "motel", "residence", "executive house", "house") {
		score -= 4
		reasons = append(reasons, "nama ambigu dan perlu verifikasi")
	}

	if strings.TrimSpace(lead.Title) == "" {
		score -= 30
		reasons = append(reasons, "nama lead kosong")
	}

	score = clampScore(score)
	status := ReviewNeedsReview
	if score >= 75 {
		status = ReviewValid
	} else if score <= 35 {
		status = ReviewExclude
	}
	return QCEvaluation{Status: status, Score: score, Reasons: reasons}
}

func evaluateGenericQC(lead Lead) QCEvaluation {
	score := 50
	reasons := make([]string, 0, 5)
	if strings.TrimSpace(lead.Title) != "" {
		score += 12
	} else {
		score -= 25
		reasons = append(reasons, "nama lead kosong")
	}
	if strings.TrimSpace(lead.Address) != "" {
		score += 10
	}
	if strings.TrimSpace(lead.Phone) != "" {
		score += 8
	}
	if strings.TrimSpace(lead.Website) != "" {
		score += 4
	}
	if lead.Rating >= 4.0 && lead.ReviewCount >= 5 {
		score += 4
	}
	score = clampScore(score)
	status := ReviewNeedsReview
	if score >= 75 {
		status = ReviewValid
	} else if score <= 35 {
		status = ReviewExclude
	}
	return QCEvaluation{Status: status, Score: score, Reasons: reasons}
}

func (s *Store) RunAutoQC(ctx context.Context, filter Filter, apply bool) (QCSummary, error) {
	if filter.Limit <= 0 || filter.Limit > 5000 {
		filter.Limit = 5000
	}
	leads, err := s.List(ctx, filter)
	if err != nil {
		return QCSummary{}, err
	}
	if err := s.EnsureReviewSchema(ctx); err != nil {
		return QCSummary{}, err
	}

	summary := QCSummary{Total: len(leads)}
	for _, lead := range leads {
		eval := EvaluateLeadQC(lead)
		switch eval.Status {
		case ReviewValid:
			summary.Valid++
		case ReviewExclude:
			summary.Exclude++
		default:
			summary.NeedsReview++
		}
		if !apply {
			continue
		}

		existing, err := s.GetReview(ctx, lead.ID)
		if err != nil {
			return summary, err
		}
		if existing.Source == ReviewSourceManual {
			summary.ManualKept++
			continue
		}
		note := fmt.Sprintf("Auto QC %d/100", eval.Score)
		if len(eval.Reasons) > 0 {
			note += " — " + strings.Join(eval.Reasons, "; ")
		}
		if err := s.updateAutoReview(ctx, lead.ID, eval.Status, note); err != nil {
			return summary, err
		}
		summary.Applied++
	}
	return summary, nil
}

func normalizeText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer("-", " ", "_", " ", "/", " ", ".", " ", ",", " ", "(", " ", ")", " ")
	return strings.Join(strings.Fields(replacer.Replace(value)), " ")
}

func containsAnyPhrase(value string, phrases ...string) bool {
	for _, phrase := range phrases {
		if strings.Contains(value, normalizeText(phrase)) {
			return true
		}
	}
	return false
}

func containsAnyWord(value string, words ...string) bool {
	fields := strings.Fields(value)
	for _, field := range fields {
		for _, word := range words {
			if field == normalizeText(word) {
				return true
			}
		}
	}
	return false
}

func clampScore(score int) int {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}
