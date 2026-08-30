package leadstore

import (
	"context"
	"database/sql"
	"fmt"
)

func (s *Store) Get(ctx context.Context, id int64) (Lead, error) {
	const query = `SELECT id,preset,area,subarea,place_id,data_id,title,category,address,phone,website,latitude,longitude,rating,review_count,link,thumbnail,first_seen,last_checked FROM leads WHERE id = ?`

	var lead Lead
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&lead.ID,
		&lead.Preset,
		&lead.Area,
		&lead.Subarea,
		&lead.PlaceID,
		&lead.DataID,
		&lead.Title,
		&lead.Category,
		&lead.Address,
		&lead.Phone,
		&lead.Website,
		&lead.Latitude,
		&lead.Longitude,
		&lead.Rating,
		&lead.ReviewCount,
		&lead.Link,
		&lead.Thumbnail,
		&lead.FirstSeen,
		&lead.LastChecked,
	)
	if err == sql.ErrNoRows {
		return Lead{}, fmt.Errorf("lead %d not found", id)
	}
	if err != nil {
		return Lead{}, fmt.Errorf("get lead: %w", err)
	}
	return lead, nil
}
