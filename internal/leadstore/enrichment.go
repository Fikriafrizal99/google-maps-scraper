package leadstore

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const (
	EnrichmentUnknown = "unknown"
	EnrichmentAuto    = "auto"
	EnrichmentManual  = "manual"

	VerificationUnverified = "unverified"
	VerificationVerified   = "verified"
	VerificationNeedsCheck = "needs_check"
)

type KostEnrichment struct {
	LeadID             int64  `json:"lead_id"`
	Segment            string `json:"segment"`
	Target             string `json:"target"`
	RentalType         string `json:"rental_type"`
	PriceRange         string `json:"price_range"`
	Facilities         string `json:"facilities"`
	Furnish            string `json:"furnish"`
	Rules              string `json:"rules"`
	Landmark           string `json:"landmark"`
	SellingPoint       string `json:"selling_point"`
	VerificationStatus string `json:"verification_status"`
	Source             string `json:"source"`
	UpdatedAt          string `json:"updated_at"`
}

func defaultKostEnrichment(leadID int64) KostEnrichment {
	return KostEnrichment{
		LeadID:             leadID,
		Segment:            EnrichmentUnknown,
		Target:             EnrichmentUnknown,
		RentalType:         EnrichmentUnknown,
		PriceRange:         EnrichmentUnknown,
		Facilities:         EnrichmentUnknown,
		Furnish:            EnrichmentUnknown,
		Rules:              EnrichmentUnknown,
		Landmark:           EnrichmentUnknown,
		SellingPoint:       EnrichmentUnknown,
		VerificationStatus: VerificationUnverified,
		Source:             EnrichmentAuto,
	}
}

func (s *Store) EnsureEnrichmentSchema(ctx context.Context) error {
	const schema = `
CREATE TABLE IF NOT EXISTS lead_enrichment (
    lead_id INTEGER PRIMARY KEY,
    segment TEXT NOT NULL DEFAULT 'unknown',
    target TEXT NOT NULL DEFAULT 'unknown',
    rental_type TEXT NOT NULL DEFAULT 'unknown',
    price_range TEXT NOT NULL DEFAULT 'unknown',
    facilities TEXT NOT NULL DEFAULT 'unknown',
    furnish TEXT NOT NULL DEFAULT 'unknown',
    rules TEXT NOT NULL DEFAULT 'unknown',
    landmark TEXT NOT NULL DEFAULT 'unknown',
    selling_point TEXT NOT NULL DEFAULT 'unknown',
    verification_status TEXT NOT NULL DEFAULT 'unverified',
    source TEXT NOT NULL DEFAULT 'auto',
    updated_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_lead_enrichment_segment ON lead_enrichment(segment);
CREATE INDEX IF NOT EXISTS idx_lead_enrichment_target ON lead_enrichment(target);
CREATE INDEX IF NOT EXISTS idx_lead_enrichment_verification ON lead_enrichment(verification_status);
`
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("migrate lead enrichment store: %w", err)
	}
	return nil
}

func normalizeEnrichmentValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return EnrichmentUnknown
	}
	return value
}

func normalizeVerificationStatus(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		value = VerificationUnverified
	}
	switch value {
	case VerificationUnverified, VerificationVerified, VerificationNeedsCheck:
		return value, nil
	default:
		return "", fmt.Errorf("invalid verification status %q", value)
	}
}

func (s *Store) GetEnrichment(ctx context.Context, leadID int64) (KostEnrichment, error) {
	if leadID <= 0 {
		return KostEnrichment{}, fmt.Errorf("invalid lead id")
	}
	if err := s.EnsureEnrichmentSchema(ctx); err != nil {
		return KostEnrichment{}, err
	}

	var e KostEnrichment
	err := s.db.QueryRowContext(ctx, `
SELECT lead_id,segment,target,rental_type,price_range,facilities,furnish,rules,landmark,
       selling_point,verification_status,source,updated_at
FROM lead_enrichment WHERE lead_id = ?`, leadID).Scan(
		&e.LeadID, &e.Segment, &e.Target, &e.RentalType, &e.PriceRange, &e.Facilities,
		&e.Furnish, &e.Rules, &e.Landmark, &e.SellingPoint, &e.VerificationStatus,
		&e.Source, &e.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return defaultKostEnrichment(leadID), nil
	}
	if err != nil {
		return KostEnrichment{}, fmt.Errorf("get lead enrichment: %w", err)
	}
	return e, nil
}

func (s *Store) UpdateEnrichment(ctx context.Context, e KostEnrichment) error {
	if e.LeadID <= 0 {
		return fmt.Errorf("invalid lead id")
	}
	verification, err := normalizeVerificationStatus(e.VerificationStatus)
	if err != nil {
		return err
	}
	if err := s.EnsureEnrichmentSchema(ctx); err != nil {
		return err
	}

	e.Segment = normalizeEnrichmentValue(e.Segment)
	e.Target = normalizeEnrichmentValue(e.Target)
	e.RentalType = normalizeEnrichmentValue(e.RentalType)
	e.PriceRange = normalizeEnrichmentValue(e.PriceRange)
	e.Facilities = normalizeEnrichmentValue(e.Facilities)
	e.Furnish = normalizeEnrichmentValue(e.Furnish)
	e.Rules = normalizeEnrichmentValue(e.Rules)
	e.Landmark = normalizeEnrichmentValue(e.Landmark)
	e.SellingPoint = normalizeEnrichmentValue(e.SellingPoint)
	e.VerificationStatus = verification
	e.Source = EnrichmentManual
	e.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	_, err = s.db.ExecContext(ctx, `
INSERT INTO lead_enrichment (
    lead_id,segment,target,rental_type,price_range,facilities,furnish,rules,landmark,
    selling_point,verification_status,source,updated_at
) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(lead_id) DO UPDATE SET
    segment=excluded.segment,
    target=excluded.target,
    rental_type=excluded.rental_type,
    price_range=excluded.price_range,
    facilities=excluded.facilities,
    furnish=excluded.furnish,
    rules=excluded.rules,
    landmark=excluded.landmark,
    selling_point=excluded.selling_point,
    verification_status=excluded.verification_status,
    source=excluded.source,
    updated_at=excluded.updated_at`,
		e.LeadID, e.Segment, e.Target, e.RentalType, e.PriceRange, e.Facilities,
		e.Furnish, e.Rules, e.Landmark, e.SellingPoint, e.VerificationStatus,
		e.Source, e.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update lead enrichment: %w", err)
	}
	return nil
}

func (s *Store) updateAutoEnrichment(ctx context.Context, e KostEnrichment) error {
	if e.LeadID <= 0 {
		return fmt.Errorf("invalid lead id")
	}
	if err := s.EnsureEnrichmentSchema(ctx); err != nil {
		return err
	}
	e.Segment = normalizeEnrichmentValue(e.Segment)
	e.Target = normalizeEnrichmentValue(e.Target)
	e.RentalType = normalizeEnrichmentValue(e.RentalType)
	e.PriceRange = normalizeEnrichmentValue(e.PriceRange)
	e.Facilities = normalizeEnrichmentValue(e.Facilities)
	e.Furnish = normalizeEnrichmentValue(e.Furnish)
	e.Rules = normalizeEnrichmentValue(e.Rules)
	e.Landmark = normalizeEnrichmentValue(e.Landmark)
	e.SellingPoint = normalizeEnrichmentValue(e.SellingPoint)
	if e.VerificationStatus == "" {
		e.VerificationStatus = VerificationUnverified
	}
	e.Source = EnrichmentAuto
	e.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	_, err := s.db.ExecContext(ctx, `
INSERT INTO lead_enrichment (
    lead_id,segment,target,rental_type,price_range,facilities,furnish,rules,landmark,
    selling_point,verification_status,source,updated_at
) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(lead_id) DO UPDATE SET
    segment=excluded.segment,
    target=excluded.target,
    rental_type=excluded.rental_type,
    price_range=excluded.price_range,
    facilities=excluded.facilities,
    furnish=excluded.furnish,
    rules=excluded.rules,
    landmark=excluded.landmark,
    selling_point=excluded.selling_point,
    verification_status=excluded.verification_status,
    source=excluded.source,
    updated_at=excluded.updated_at
WHERE lead_enrichment.source != 'manual'`,
		e.LeadID, e.Segment, e.Target, e.RentalType, e.PriceRange, e.Facilities,
		e.Furnish, e.Rules, e.Landmark, e.SellingPoint, e.VerificationStatus,
		e.Source, e.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update auto enrichment: %w", err)
	}
	return nil
}

func (s *Store) EnrichmentMap(ctx context.Context, leadIDs []int64) (map[int64]KostEnrichment, error) {
	if err := s.EnsureEnrichmentSchema(ctx); err != nil {
		return nil, err
	}
	result := make(map[int64]KostEnrichment, len(leadIDs))
	for _, id := range leadIDs {
		result[id] = defaultKostEnrichment(id)
	}
	if len(leadIDs) == 0 {
		return result, nil
	}

	const chunkSize = 500
	for start := 0; start < len(leadIDs); start += chunkSize {
		end := start + chunkSize
		if end > len(leadIDs) {
			end = len(leadIDs)
		}
		ids := leadIDs[start:end]
		placeholders := make([]string, len(ids))
		args := make([]any, len(ids))
		for i, id := range ids {
			placeholders[i] = "?"
			args[i] = id
		}
		query := `
SELECT lead_id,segment,target,rental_type,price_range,facilities,furnish,rules,landmark,
       selling_point,verification_status,source,updated_at
FROM lead_enrichment WHERE lead_id IN (` + strings.Join(placeholders, ",") + `)`
		rows, err := s.db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, fmt.Errorf("list lead enrichment: %w", err)
		}
		for rows.Next() {
			var e KostEnrichment
			if err := rows.Scan(
				&e.LeadID, &e.Segment, &e.Target, &e.RentalType, &e.PriceRange, &e.Facilities,
				&e.Furnish, &e.Rules, &e.Landmark, &e.SellingPoint, &e.VerificationStatus,
				&e.Source, &e.UpdatedAt,
			); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan lead enrichment: %w", err)
			}
			result[e.LeadID] = e
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, fmt.Errorf("list lead enrichment rows: %w", err)
		}
		rows.Close()
	}
	return result, nil
}
