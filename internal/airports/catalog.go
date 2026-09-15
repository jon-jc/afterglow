// Package airports owns external airport reference data, independently of
// campaign money and synthetic advertising inventory.
package airports

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const SourceURL = "https://davidmegginson.github.io/ourairports-data/airports.csv"
const Scope = "US and US territories; scheduled_service=yes; non-closed facilities. Source-defined airline service, not FAA commercial-service certification."
const maxDownload = 32 << 20

type Airport struct {
	Ident   string  `json:"ident"`
	IATA    string  `json:"iata"`
	Name    string  `json:"name"`
	City    string  `json:"city"`
	Region  string  `json:"region"`
	Country string  `json:"country"`
	Type    string  `json:"type"`
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
}
type Snapshot struct {
	Airports       []Airport `json:"airports"`
	FetchedAt      time.Time `json:"fetched_at"`
	SourceModified string    `json:"source_modified"`
	SHA256         string    `json:"sha256"`
}
type Catalog struct {
	mu        sync.RWMutex
	refreshMu sync.Mutex
	snapshot  Snapshot
	lastErr   string
	CachePath string
	Source    string
	Client    *http.Client
}

func New(cache string) (*Catalog, error) {
	c := &Catalog{CachePath: cache, Source: SourceURL, Client: &http.Client{Timeout: 30 * time.Second}}
	b, err := os.ReadFile(cache)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(b, &c.snapshot); err != nil {
		return nil, fmt.Errorf("invalid airport cache: %w", err)
	}
	if err = validate(c.snapshot.Airports); err != nil {
		return nil, err
	}
	return c, nil
}
func (c *Catalog) read() (Snapshot, string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.snapshot, c.lastErr // published slices are immutable
}
func (c *Catalog) Refresh(ctx context.Context) (err error) {
	c.refreshMu.Lock()
	defer c.refreshMu.Unlock()
	defer func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if err != nil {
			c.lastErr = "Source refresh failed; retaining last successful snapshot"
		} else {
			c.lastErr = ""
		}
	}()
	req, err := http.NewRequestWithContext(ctx, "GET", c.Source, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Afterglow-Airport-Catalog/1.0")
	res, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("airport source returned %d", res.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, maxDownload+1))
	if err != nil {
		return err
	}
	if len(b) > maxDownload {
		return errors.New("airport download exceeds limit")
	}
	a, err := parse(strings.NewReader(string(b)))
	if err != nil {
		return err
	}
	// A truncated but syntactically valid file must not silently erase the catalog.
	if len(a) < 100 {
		return errors.New("airport source has implausibly few qualifying records")
	}
	h := sha256.Sum256(b)
	next := Snapshot{Airports: a, FetchedAt: time.Now().UTC(), SourceModified: res.Header.Get("Last-Modified"), SHA256: hex.EncodeToString(h[:])}
	if err = persist(c.CachePath, next); err != nil {
		return err
	}
	c.mu.Lock()
	c.snapshot = next
	c.mu.Unlock()
	return nil
}
func persist(path string, data Snapshot) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), "airports-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = json.NewEncoder(f).Encode(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
func parse(r io.Reader) ([]Airport, error) {
	reader := csv.NewReader(r)
	header, err := reader.Read()
	if err != nil {
		return nil, err
	}
	cols := map[string]int{}
	for i, k := range header {
		cols[k] = i
	}
	for _, k := range []string{"ident", "iata_code", "name", "municipality", "iso_region", "iso_country", "type", "latitude_deg", "longitude_deg", "scheduled_service"} {
		if _, ok := cols[k]; !ok {
			return nil, fmt.Errorf("missing column %s", k)
		}
	}
	var out []Airport
	for {
		row, e := reader.Read()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, e
		}
		get := func(k string) string { return row[cols[k]] }
		country := get("iso_country")
		if !usCountry(country) || get("scheduled_service") != "yes" || strings.HasPrefix(get("type"), "closed") {
			continue
		}
		lat, e := strconv.ParseFloat(get("latitude_deg"), 64)
		if e != nil {
			return nil, e
		}
		lng, e := strconv.ParseFloat(get("longitude_deg"), 64)
		if e != nil {
			return nil, e
		}
		out = append(out, Airport{Ident: get("ident"), IATA: get("iata_code"), Name: get("name"), City: get("municipality"), Region: get("iso_region"), Country: country, Type: get("type"), Lat: lat, Lng: lng})
	}
	if err = validate(out); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Ident < out[j].Ident })
	return out, nil
}
func usCountry(s string) bool {
	switch s {
	case "US", "PR", "VI", "GU", "AS", "MP", "UM":
		return true
	}
	return false
}
func validate(a []Airport) error {
	seen := map[string]bool{}
	for _, v := range a {
		if v.Ident == "" || v.Name == "" || seen[v.Ident] || !usCountry(v.Country) || math.IsNaN(v.Lat) || math.IsNaN(v.Lng) || math.IsInf(v.Lat, 0) || math.IsInf(v.Lng, 0) || math.Abs(v.Lat) > 90 || math.Abs(v.Lng) > 180 {
			return errors.New("invalid or duplicate airport record")
		}
		seen[v.Ident] = true
	}
	return nil
}

func (c *Catalog) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]string{"status": "alive"}) })
	mux.HandleFunc("GET /v1/airports", func(w http.ResponseWriter, r *http.Request) {
		s, warning := c.read()
		if len(s.Airports) == 0 {
			writeJSON(w, 503, map[string]string{"detail": "Airport catalog is loading; retry shortly"})
			return
		}
		q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
		region := r.URL.Query().Get("region")
		if len(q) > 100 || len(region) > 20 {
			writeJSON(w, 400, map[string]string{"detail": "Search exceeds maximum length"})
			return
		}
		limit, offset := 50, 0
		for key, dst := range map[string]*int{"limit": &limit, "offset": &offset} {
			if raw := r.URL.Query().Get(key); raw != "" {
				n, err := strconv.Atoi(raw)
				if err != nil || n < 0 {
					writeJSON(w, 400, map[string]string{"detail": "Invalid pagination"})
					return
				}
				*dst = n
			}
		}
		if limit < 1 || limit > 200 {
			writeJSON(w, 400, map[string]string{"detail": "Limit must be 1 to 200"})
			return
		}
		matched := make([]Airport, 0)
		regions := map[string]bool{}
		for _, a := range s.Airports {
			regions[a.Region] = true
			if region != "" && a.Region != region {
				continue
			}
			if q != "" && !strings.Contains(strings.ToLower(a.Ident+" "+a.IATA+" "+a.Name+" "+a.City+" "+a.Region), q) {
				continue
			}
			matched = append(matched, a)
		}
		keys := make([]string, 0, len(regions))
		for k := range regions {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if offset > len(matched) {
			offset = len(matched)
		}
		end := min(offset+limit, len(matched))
		writeJSON(w, 200, map[string]any{"airports": matched[offset:end], "map_airports": matched, "matched": len(matched), "total": len(s.Airports), "offset": offset, "limit": limit, "regions": keys, "source": SourceURL, "scope": Scope, "fetched_at": s.FetchedAt, "source_modified": s.SourceModified, "version": s.SHA256, "stale": warning != "" || time.Since(s.FetchedAt) > 48*time.Hour, "warning": warning})
	})
	return mux
}
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
