package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// readOnlyTransport lässt nur GET-Anfragen durch. Das Werkzeug soll in Ninox
// nie etwas ändern können – auch nicht durch einen Programmierfehler.
type readOnlyTransport struct{ next http.RoundTripper }

func (t readOnlyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodGet {
		return nil, fmt.Errorf("nur lesende Anfragen erlaubt, %s blockiert", req.Method)
	}
	return t.next.RoundTrip(req)
}

// Stats sammelt Antwortzeiten und Statuscodes (für die Frage nach
// Rate-Limits und Antwortzeiten in #5).
type Stats struct {
	Durations []time.Duration
	Status    map[int]int
	Retries   int
}

func (s *Stats) add(d time.Duration, status int) {
	if s.Status == nil {
		s.Status = map[int]int{}
	}
	s.Durations = append(s.Durations, d)
	s.Status[status]++
}

// Percentile liefert das p-Quantil (0..1) der Antwortzeiten.
func (s *Stats) Percentile(p float64) time.Duration {
	if len(s.Durations) == 0 {
		return 0
	}
	d := append([]time.Duration(nil), s.Durations...)
	sort.Slice(d, func(i, j int) bool { return d[i] < d[j] })
	i := int(p*float64(len(d)-1) + 0.5)
	return d[i]
}

// HTTPError ist eine Antwort mit Status >= 400.
type HTTPError struct {
	Status int
	Path   string
	Body   string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("GET %s: HTTP %d: %s", e.Path, e.Status, e.Body)
}

// Client spricht die Ninox-API ausschließlich lesend an.
type Client struct {
	base    string
	token   string
	hc      *http.Client
	pause   time.Duration // Pause zwischen Anfragen (Ninox begrenzt parallele Aufrufe)
	retries int
	Stats   Stats
}

func NewClient(base, token string, pause time.Duration) *Client {
	return &Client{
		base:    strings.TrimRight(base, "/"),
		token:   token,
		hc:      &http.Client{Timeout: 60 * time.Second, Transport: readOnlyTransport{http.DefaultTransport}},
		pause:   pause,
		retries: 4,
	}
}

// Get holt path (relativ zur Basis-URL) und dekodiert JSON nach out.
// Zahlen bleiben als json.Number erhalten, damit rein numerische
// Maschinennummern nicht als 1.2345678e+07 enden.
// Bei 429/502/503/504 wird mit Pause wiederholt (Retry-After wird beachtet).
func (c *Client) Get(ctx context.Context, path string, q url.Values, out any) error {
	u := c.base + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	var lastErr error
	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 {
			c.Stats.Retries++
		}
		if c.pause > 0 {
			time.Sleep(c.pause)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept", "application/json")

		start := time.Now()
		resp, err := c.hc.Do(req)
		if err != nil {
			return fmt.Errorf("GET %s: %w", path, err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		c.Stats.add(time.Since(start), resp.StatusCode)
		if err != nil {
			return fmt.Errorf("GET %s: %w", path, err)
		}

		switch resp.StatusCode {
		case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			lastErr = &HTTPError{Status: resp.StatusCode, Path: path, Body: shorten(string(body))}
			time.Sleep(backoff(resp.Header.Get("Retry-After"), attempt))
			continue
		}
		if resp.StatusCode >= 400 {
			return &HTTPError{Status: resp.StatusCode, Path: path, Body: shorten(string(body))}
		}
		dec := json.NewDecoder(bytes.NewReader(body))
		dec.UseNumber()
		if err := dec.Decode(out); err != nil {
			return fmt.Errorf("GET %s: Antwort ist kein erwartetes JSON: %w (Anfang: %q)", path, err, shorten(string(body)))
		}
		return nil
	}
	return lastErr
}

func backoff(retryAfter string, attempt int) time.Duration {
	if s, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil && s >= 0 && s < 120 {
		return time.Duration(s) * time.Second
	}
	return time.Duration(1<<attempt) * time.Second
}

func shorten(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}

// IsStatus prüft, ob err ein HTTPError mit einem der Statuscodes ist.
func IsStatus(err error, codes ...int) bool {
	var he *HTTPError
	if !errors.As(err, &he) {
		return false
	}
	for _, c := range codes {
		if he.Status == c {
			return true
		}
	}
	return false
}
