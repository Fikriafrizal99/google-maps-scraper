package geodata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const DefaultBaseURL = "https://www.emsifa.com/api-wilayah-indonesia/v2"

type Region struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Client struct {
	baseURL    string
	cacheDir   string
	httpClient *http.Client
	mu         sync.Mutex
}

var javaSumatraProvinceIDs = map[string]struct{}{
	"11": {}, "12": {}, "13": {}, "14": {}, "15": {}, "16": {}, "17": {}, "18": {}, "19": {}, "21": {},
	"31": {}, "32": {}, "33": {}, "34": {}, "35": {}, "36": {},
}

// Emsifa v2 uses dotted hierarchy IDs, for example:
// 32 -> 32.01 -> 32.01.01 -> 32.01.01.2001.
var regionID = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+){0,3}$`)

func New(cacheDir string) *Client {
	return NewWithBaseURL(cacheDir, DefaultBaseURL)
}

func NewWithBaseURL(cacheDir, baseURL string) *Client {
	return &Client{
		baseURL:  strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		cacheDir: cacheDir,
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func (c *Client) Provinces(ctx context.Context) ([]Region, error) {
	regions, err := c.load(ctx, "v2-provinces.json", "provinces.json")
	if err != nil {
		return nil, err
	}
	filtered := make([]Region, 0, 16)
	for _, region := range regions {
		if _, ok := javaSumatraProvinceIDs[region.ID]; ok {
			filtered = append(filtered, region)
		}
	}
	return filtered, nil
}

func (c *Client) Regencies(ctx context.Context, provinceID string) ([]Region, error) {
	provinceID = strings.TrimSpace(provinceID)
	if _, ok := javaSumatraProvinceIDs[provinceID]; !ok {
		return nil, fmt.Errorf("province %q is outside Java-Sumatra scope", provinceID)
	}
	return c.load(ctx, "v2-regencies-"+provinceID+".json", "regencies/"+provinceID+".json")
}

func (c *Client) Districts(ctx context.Context, regencyID string) ([]Region, error) {
	regencyID = strings.TrimSpace(regencyID)
	if !regionID.MatchString(regencyID) {
		return nil, fmt.Errorf("invalid regency id")
	}
	return c.load(ctx, "v2-districts-"+cacheSafeID(regencyID)+".json", "districts/"+regencyID+".json")
}

func (c *Client) Villages(ctx context.Context, districtID string) ([]Region, error) {
	districtID = strings.TrimSpace(districtID)
	if !regionID.MatchString(districtID) {
		return nil, fmt.Errorf("invalid district id")
	}
	return c.load(ctx, "v2-villages-"+cacheSafeID(districtID)+".json", "villages/"+districtID+".json")
}

func cacheSafeID(id string) string {
	return strings.ReplaceAll(id, ".", "-")
}

func (c *Client) load(ctx context.Context, cacheName, upstreamPath string) ([]Region, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if strings.TrimSpace(c.cacheDir) != "" {
		cachePath := filepath.Join(c.cacheDir, cacheName)
		if data, err := os.ReadFile(cachePath); err == nil {
			if regions, decodeErr := decodeRegions(data); decodeErr == nil && len(regions) > 0 {
				return regions, nil
			}
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/"+upstreamPath, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download geo data: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download geo data: upstream returned %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("read geo data: %w", err)
	}
	regions, err := decodeRegions(data)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(c.cacheDir) != "" {
		if err := os.MkdirAll(c.cacheDir, 0o755); err == nil {
			_ = os.WriteFile(filepath.Join(c.cacheDir, cacheName), data, 0o644)
		}
	}
	return regions, nil
}

func decodeRegions(data []byte) ([]Region, error) {
	// Keep compatibility with bare-array fixtures or an explicitly supplied legacy API.
	var regions []Region
	if err := json.Unmarshal(data, &regions); err == nil && len(regions) > 0 {
		return regions, nil
	}

	// Emsifa v2 wraps the payload in {"data": [...], "meta": {...}}.
	var wrapped struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return nil, fmt.Errorf("decode geo data: %w", err)
	}
	if len(wrapped.Data) == 0 || string(wrapped.Data) == "null" {
		return nil, fmt.Errorf("geo data response has no data")
	}
	if err := json.Unmarshal(wrapped.Data, &regions); err != nil {
		return nil, fmt.Errorf("decode geo data payload: %w", err)
	}
	if len(regions) == 0 {
		return nil, fmt.Errorf("geo data is empty")
	}
	return regions, nil
}
