package leadstore

import (
	"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type Lead struct {
	ID          int64   `json:"id"`
	Preset      string  `json:"preset"`
	Area        string  `json:"area"`
	Subarea     string  `json:"subarea"`
	PlaceID     string  `json:"place_id"`
	DataID      string  `json:"data_id"`
	Title       string  `json:"title"`
	Category    string  `json:"category"`
	Address     string  `json:"address"`
	Phone       string  `json:"phone"`
	Website     string  `json:"website"`
	Latitude    string  `json:"latitude"`
	Longitude   string  `json:"longitude"`
	Rating      float64 `json:"rating"`
	ReviewCount int     `json:"review_count"`
	Link        string  `json:"link"`
	Thumbnail   string  `json:"thumbnail"`
	Images      string  `json:"images"`
	FirstSeen   string  `json:"first_seen"`
	LastChecked string  `json:"last_checked"`
}

type Filter struct {
	Preset    string
	Area      string
	Subarea   string
	Query     string
	HasPhone  bool
	MinRating float64
	Limit     int
}

type Stats struct {
	Total       int     `json:"total"`
	WithPhone   int     `json:"with_phone"`
	WithWebsite int     `json:"with_website"`
	AvgRating   float64 `json:"avg_rating"`
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	store := &Store{db: db}
	if err := store.migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate(ctx context.Context) error {
	const schema = `
CREATE TABLE IF NOT EXISTS leads (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_key TEXT NOT NULL UNIQUE,
    preset TEXT NOT NULL,
    area TEXT NOT NULL,
    subarea TEXT NOT NULL DEFAULT '',
    place_id TEXT NOT NULL DEFAULT '',
    data_id TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL DEFAULT '',
    address TEXT NOT NULL DEFAULT '',
    phone TEXT NOT NULL DEFAULT '',
    website TEXT NOT NULL DEFAULT '',
    latitude TEXT NOT NULL DEFAULT '',
    longitude TEXT NOT NULL DEFAULT '',
    rating REAL NOT NULL DEFAULT 0,
    review_count INTEGER NOT NULL DEFAULT 0,
    link TEXT NOT NULL DEFAULT '',
    thumbnail TEXT NOT NULL DEFAULT '',
    images TEXT NOT NULL DEFAULT '',
    first_seen TEXT NOT NULL,
    last_checked TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_leads_preset_area ON leads(preset, area, subarea);
CREATE INDEX IF NOT EXISTS idx_leads_title ON leads(title);
CREATE INDEX IF NOT EXISTS idx_leads_phone ON leads(phone);
CREATE INDEX IF NOT EXISTS idx_leads_rating ON leads(rating);
`
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("migrate lead store: %w", err)
	}
	if err := s.ensureColumn(ctx, "leads", "images", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	return nil
}

func (s *Store) ensureColumn(ctx context.Context, table, column, definition string) error {
	rows, err := s.db.QueryContext(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		return fmt.Errorf("inspect %s schema: %w", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultValue, &pk); err != nil {
			return fmt.Errorf("scan %s schema: %w", table, err)
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("inspect %s schema rows: %w", table, err)
	}
	if _, err := s.db.ExecContext(ctx, "ALTER TABLE "+table+" ADD COLUMN "+column+" "+definition); err != nil {
		return fmt.Errorf("add %s.%s column: %w", table, column, err)
	}
	return nil
}

func (s *Store) ImportCSV(ctx context.Context, path, preset, area, subarea string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open import csv: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	header, err := reader.Read()
	if err != nil {
		return 0, fmt.Errorf("read csv header: %w", err)
	}
	index := make(map[string]int, len(header))
	for i, name := range header {
		index[strings.ToLower(strings.TrimSpace(name))] = i
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin import: %w", err)
	}
	defer tx.Rollback()

	const upsert = `
INSERT INTO leads (
    source_key,preset,area,subarea,place_id,data_id,title,category,address,phone,website,
    latitude,longitude,rating,review_count,link,thumbnail,images,first_seen,last_checked
) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(source_key) DO UPDATE SET
    area=excluded.area, subarea=excluded.subarea,
    place_id=excluded.place_id, data_id=excluded.data_id, title=excluded.title,
    category=excluded.category, address=excluded.address, phone=excluded.phone,
    website=excluded.website, latitude=excluded.latitude, longitude=excluded.longitude,
    rating=excluded.rating, review_count=excluded.review_count, link=excluded.link,
    thumbnail=excluded.thumbnail, images=excluded.images, last_checked=excluded.last_checked`

	stmt, err := tx.PrepareContext(ctx, upsert)
	if err != nil {
		return 0, fmt.Errorf("prepare import: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	count := 0
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, fmt.Errorf("read csv row: %w", err)
		}

		get := func(names ...string) string {
			for _, name := range names {
				if i, ok := index[name]; ok && i < len(record) {
					return strings.TrimSpace(record[i])
				}
			}
			return ""
		}

		placeID := get("place_id")
		dataID := get("data_id")
		link := get("link")
		title := get("title", "name")
		lat := get("latitude")
		lng := get("longitude", "longtitude")
		identity := firstNonEmpty(placeID, dataID, link)
		if identity == "" {
			identity = strings.ToLower(strings.TrimSpace(title + "|" + lat + "|" + lng))
		}
		if identity == "" {
			continue
		}
		sourceKey := strings.ToLower(strings.TrimSpace(preset)) + "|" + identity

		rating, _ := strconv.ParseFloat(get("review_rating", "rating"), 64)
		reviews, _ := strconv.Atoi(get("review_count", "reviews"))
		if _, err := stmt.ExecContext(ctx,
			sourceKey, preset, area, subarea, placeID, dataID, title, get("category"), get("address"),
			get("phone"), get("website", "web_site"), lat, lng, rating, reviews, link, get("thumbnail"), get("images"), now, now,
		); err != nil {
			return 0, fmt.Errorf("upsert lead %q: %w", title, err)
		}
		count++
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit import: %w", err)
	}
	return count, nil
}

func (s *Store) List(ctx context.Context, filter Filter) ([]Lead, error) {
	query := `SELECT id,preset,area,subarea,place_id,data_id,title,category,address,phone,website,latitude,longitude,rating,review_count,link,thumbnail,images,first_seen,last_checked FROM leads WHERE 1=1`
	args := make([]any, 0, 8)
	if filter.Preset != "" {
		query += " AND preset = ?"
		args = append(args, filter.Preset)
	}
	if filter.Area != "" {
		query += " AND area = ?"
		args = append(args, filter.Area)
	}
	if filter.Subarea != "" {
		query += " AND subarea = ?"
		args = append(args, filter.Subarea)
	}
	if filter.Query != "" {
		query += " AND (title LIKE ? OR address LIKE ? OR category LIKE ? OR phone LIKE ?)"
		q := "%" + filter.Query + "%"
		args = append(args, q, q, q, q)
	}
	if filter.HasPhone {
		query += " AND TRIM(phone) <> ''"
	}
	if filter.MinRating > 0 {
		query += " AND rating >= ?"
		args = append(args, filter.MinRating)
	}
	query += " ORDER BY last_checked DESC, rating DESC, review_count DESC"
	limit := filter.Limit
	if limit <= 0 || limit > 5000 {
		limit = 500
	}
	query += " LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list leads: %w", err)
	}
	defer rows.Close()

	var leads []Lead
	for rows.Next() {
		var lead Lead
		if err := rows.Scan(&lead.ID, &lead.Preset, &lead.Area, &lead.Subarea, &lead.PlaceID, &lead.DataID, &lead.Title, &lead.Category, &lead.Address, &lead.Phone, &lead.Website, &lead.Latitude, &lead.Longitude, &lead.Rating, &lead.ReviewCount, &lead.Link, &lead.Thumbnail, &lead.Images, &lead.FirstSeen, &lead.LastChecked); err != nil {
			return nil, fmt.Errorf("scan lead: %w", err)
		}
		leads = append(leads, lead)
	}
	return leads, rows.Err()
}

func (s *Store) Stats(ctx context.Context) (Stats, error) {
	var stats Stats
	const q = `SELECT COUNT(*), COALESCE(SUM(CASE WHEN TRIM(phone) <> '' THEN 1 ELSE 0 END),0), COALESCE(SUM(CASE WHEN TRIM(website) <> '' THEN 1 ELSE 0 END),0), COALESCE(AVG(NULLIF(rating,0)),0) FROM leads`
	if err := s.db.QueryRowContext(ctx, q).Scan(&stats.Total, &stats.WithPhone, &stats.WithWebsite, &stats.AvgRating); err != nil {
		return Stats{}, fmt.Errorf("lead stats: %w", err)
	}
	return stats, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
