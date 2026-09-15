package httpapi

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// AirportCatalog exposes a fixed upstream through the existing authenticated
// API. Browser-supplied URLs are never used as upstream destinations.
func AirportCatalog(base string) http.Handler {
	client := &http.Client{Timeout: 4 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
		defer cancel()
		u, err := url.Parse(strings.TrimRight(base, "/") + "/v1/airports")
		if err != nil {
			problem(w, 503, "airport_service_not_configured")
			return
		}
		q := url.Values{}
		for _, k := range []string{"q", "region", "limit", "offset"} {
			if v := r.URL.Query().Get(k); v != "" {
				q.Set(k, v)
			}
		}
		u.RawQuery = q.Encode()
		req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
		if err != nil {
			problem(w, 503, "airport_service_not_configured")
			return
		}
		res, err := client.Do(req)
		if err != nil {
			problem(w, 503, "Airport service unavailable. Start the airport service and retry.")
			return
		}
		defer res.Body.Close()
		b, err := io.ReadAll(io.LimitReader(res.Body, (2<<20)+1))
		if err != nil || len(b) > 2<<20 || !strings.HasPrefix(res.Header.Get("Content-Type"), "application/json") || res.StatusCode >= 300 && res.StatusCode < 400 {
			problem(w, 503, "airport_service_invalid_response")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(res.StatusCode)
		_, _ = w.Write(b)
	})
}
