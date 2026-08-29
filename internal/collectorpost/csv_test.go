package collectorpost

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gosom/google-maps-scraper/internal/collectorconfig"
)

func TestProcessCSVFiltersAndDeduplicates(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	input := filepath.Join(dir, "raw.csv")
	output := filepath.Join(dir, "out.csv")

	raw := strings.Join([]string{
		"input_id,link,title,category,address,open_hours,popular_times,website,phone,plus_code,review_count,review_rating,reviews_per_rating,latitude,longitude,cid,status,descriptions,reviews_link,thumbnail,timezone,price_range,data_id,street_view_url,place_id,images,reservations,order_online,menu,owner,complete_address,credit_cards_accepted,about,user_reviews,user_reviews_extended,emails",
		"1,http://a,Kost Melati,Kost,Jl. A,,,https://a.example,0812,,10,4.5,,,-6.2,106.8,,,,,,,d1,,p1,,,,,,,,,,,",
		"2,http://b,Kost Melati Duplikat,Kost,Jl. B,,,https://b.example,0813,,11,4.6,,,-6.21,106.81,,,,,,,d2,,p1,,,,,,,,,,,",
		"3,http://c,Hotel Mawar,Hotel,Jl. C,,,https://c.example,0814,,20,4.7,,,-6.22,106.82,,,,,,,d3,,p3,,,,,,,,,,,",
	}, "\n")
	if err := os.WriteFile(input, []byte(raw), 0o600); err != nil {
		t.Fatalf("write raw csv: %v", err)
	}

	preset := collectorconfig.Preset{
		Filters: collectorconfig.FilterConfig{
			RequiredFields:       []string{"title", "address"},
			IncludeTitlePatterns: []string{"kost", "kos"},
			ExcludeTitlePatterns: []string{"hotel"},
		},
		Dedup: collectorconfig.DedupConfig{Keys: []string{"place_id", "data_id"}},
		OutputFields: []string{"place_id", "title", "address", "rating", "reviews"},
	}

	if err := ProcessCSV(input, output, preset); err != nil {
		t.Fatalf("ProcessCSV() error = %v", err)
	}

	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	text := string(data)
	if strings.Count(strings.TrimSpace(text), "\n") != 1 {
		t.Fatalf("output = %q, want header plus one row", text)
	}
	if !strings.Contains(text, "Kost Melati") {
		t.Fatalf("output missing expected kost row: %q", text)
	}
	if strings.Contains(text, "Hotel Mawar") {
		t.Fatalf("output contains excluded hotel row: %q", text)
	}
}
