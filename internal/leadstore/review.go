package leadstore

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const (
	ReviewUnreviewed  = "unreviewed"
	ReviewValid       = "valid"
	ReviewNeedsReview = "needs_review"
	ReviewExclude     = "exclude"
)

type Review struct {
	LeadID     int64  `json:"lead_id"`
	Status     string `json:"status"`
	Note       string `json:"note"`
	ReviewedAt string `json:"reviewed_at"`
}

func (s *Store) EnsureReviewSchema(ctx context.Context) error {
	const schema = `
CREATE TABLE IF NOT EXISTS lead_reviews (
    lead_id INTEGER PRIMARY KEY,
    status TEXT NOT NULL DEFAULT 'unreviewed',
    note TEXT NOT NULL DEFAULT '',
    reviewed_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_lead_reviews_status ON lead_reviews(status);
`
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("migrate lead review store: %w", err)
	}
	return nil
}

func NormalizeReviewStatus(status string) (string, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		status = ReviewUnreviewed
	}
	switch status {
	case ReviewUnreviewed, ReviewValid, ReviewNeedsReview, ReviewExclude:
		return status, nil
	default:
		return "", fmt.Errorf("invalid review status %q", status)
	}
}

func (s *Store) UpdateReview(ctx context.Context, leadID int64, status, note string) error {
	if leadID <= 0 {
		return fmt.Errorf("invalid lead id")
	}
	status, err := NormalizeReviewStatus(status)
	if err != nil {
		return err
	}
	note = strings.TrimSpace(note)
	if len(note) > 2000 {
		return fmt.Errorf("review note too long")
	}
	if err := s.EnsureReviewSchema(ctx); err != nil {
		return err
	}

	if status == ReviewUnreviewed && note == "" {
		if _, err := s.db.ExecContext(ctx, `DELETE FROM lead_reviews WHERE lead_id = ?`, leadID); err != nil {
			return fmt.Errorf("clear lead review: %w", err)
		}
		return nil
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.db.ExecContext(ctx, `
INSERT INTO lead_reviews (lead_id,status,note,reviewed_at)
VALUES (?,?,?,?)
ON CONFLICT(lead_id) DO UPDATE SET
    status=excluded.status,
    note=excluded.note,
    reviewed_at=excluded.reviewed_at`, leadID, status, note, now)
	if err != nil {
		return fmt.Errorf("update lead review: %w", err)
	}
	return nil
}

func (s *Store) GetReview(ctx context.Context, leadID int64) (Review, error) {
	if err := s.EnsureReviewSchema(ctx); err != nil {
		return Review{}, err
	}
	var review Review
	err := s.db.QueryRowContext(ctx, `SELECT lead_id,status,note,reviewed_at FROM lead_reviews WHERE lead_id = ?`, leadID).Scan(
		&review.LeadID, &review.Status, &review.Note, &review.ReviewedAt,
	)
	if err == sql.ErrNoRows {
		return Review{LeadID: leadID, Status: ReviewUnreviewed}, nil
	}
	if err != nil {
		return Review{}, fmt.Errorf("get lead review: %w", err)
	}
	return review, nil
}

func (s *Store) ReviewMap(ctx context.Context, leadIDs []int64) (map[int64]Review, error) {
	if err := s.EnsureReviewSchema(ctx); err != nil {
		return nil, err
	}
	result := make(map[int64]Review, len(leadIDs))
	for _, id := range leadIDs {
		result[id] = Review{LeadID: id, Status: ReviewUnreviewed}
	}
	if len(leadIDs) == 0 {
		return result, nil
	}

	placeholders := make([]string, len(leadIDs))
	args := make([]any, len(leadIDs))
	for i, id := range leadIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	query := `SELECT lead_id,status,note,reviewed_at FROM lead_reviews WHERE lead_id IN (` + strings.Join(placeholders, ",") + `)`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list lead reviews: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var review Review
		if err := rows.Scan(&review.LeadID, &review.Status, &review.Note, &review.ReviewedAt); err != nil {
			return nil, fmt.Errorf("scan lead review: %w", err)
		}
		result[review.LeadID] = review
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list lead review rows: %w", err)
	}
	return result, nil
}
