package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Eintrag ist ein Protokolleintrag: entweder eine HTTP-Anfrage (Art "http")
// oder eine MCP-Nachricht (Art "mcp"), die innerhalb einer HTTP-Anfrage an /mcp kam.
type Eintrag struct {
	Nr         int64               `json:"nr"`
	Zeit       time.Time           `json:"zeit"`
	Art        string              `json:"art"`
	Methode    string              `json:"methode"`
	Pfad       string              `json:"pfad,omitempty"`
	Query      map[string][]string `json:"query,omitempty"`
	Header     map[string][]string `json:"header,omitempty"`
	Body       any                 `json:"body,omitempty"`
	RemoteAddr string              `json:"remote_addr,omitempty"`
	Status     int                 `json:"status,omitempty"`
	Antwort    any                 `json:"antwort,omitempty"`
	DauerMs    int64               `json:"dauer_ms"`
	Notizen    map[string]any      `json:"notizen,omitempty"`
}

// Protokoll hält die letzten Einträge im Speicher (Ringpuffer) und schreibt
// jeden Eintrag zusätzlich als JSON-Zeile nach stdout.
type Protokoll struct {
	mu      sync.Mutex
	max     int
	naechst int64
	liste   []Eintrag
	aus     io.Writer
}

func NeuesProtokoll(max int, aus io.Writer) *Protokoll {
	if aus == nil {
		aus = os.Stdout
	}
	return &Protokoll{max: max, aus: aus}
}

func (p *Protokoll) Hinzufuegen(e Eintrag) Eintrag {
	p.mu.Lock()
	p.naechst++
	e.Nr = p.naechst
	p.liste = append(p.liste, e)
	if len(p.liste) > p.max {
		p.liste = p.liste[len(p.liste)-p.max:]
	}
	p.mu.Unlock()

	zeile, err := json.Marshal(e)
	if err != nil {
		zeile = []byte(fmt.Sprintf(`{"fehler":%q}`, err.Error()))
	}
	p.mu.Lock()
	_, _ = p.aus.Write(append(zeile, '\n'))
	p.mu.Unlock()
	return e
}

// Einträge liefert eine Kopie, gefiltert nach Zeitpunkt und Pfad-Präfix.
func (p *Protokoll) Eintraege(seit time.Time, pfad string) []Eintrag {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]Eintrag, 0, len(p.liste))
	for _, e := range p.liste {
		if !seit.IsZero() && e.Zeit.Before(seit) {
			continue
		}
		if pfad != "" && !strings.HasPrefix(e.Pfad, pfad) {
			continue
		}
		out = append(out, e)
	}
	return out
}

func (p *Protokoll) Leeren() {
	p.mu.Lock()
	p.liste = nil
	p.mu.Unlock()
}

// --- Notizen: Handler können zum laufenden Eintrag Auswertungshinweise ergänzen ---

type notizenKey struct{}

type notizen struct {
	mu sync.Mutex
	m  map[string]any
}

func mitNotizen(ctx context.Context) (context.Context, *notizen) {
	n := &notizen{m: map[string]any{}}
	return context.WithValue(ctx, notizenKey{}, n), n
}

// notiz hängt eine Auswertungsnotiz an den Protokolleintrag der laufenden Anfrage.
func notiz(ctx context.Context, schluessel string, wert any) {
	n, ok := ctx.Value(notizenKey{}).(*notizen)
	if !ok {
		return
	}
	n.mu.Lock()
	n.m[schluessel] = wert
	n.mu.Unlock()
}

func (n *notizen) kopie() map[string]any {
	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.m) == 0 {
		return nil
	}
	out := make(map[string]any, len(n.m))
	for k, v := range n.m {
		out[k] = v
	}
	return out
}

// --- HTTP-Middleware: jede Anfrage vollständig protokollieren ---

const maxBodyProtokoll = 1 << 20 // 1 MiB

type mitschnitt struct {
	http.ResponseWriter
	status int
	buf    bytes.Buffer
}

func (m *mitschnitt) WriteHeader(code int) {
	if m.status == 0 {
		m.status = code
	}
	m.ResponseWriter.WriteHeader(code)
}

func (m *mitschnitt) Write(b []byte) (int, error) {
	if m.status == 0 {
		m.status = http.StatusOK
	}
	if m.buf.Len() < maxBodyProtokoll {
		m.buf.Write(b)
	}
	return m.ResponseWriter.Write(b)
}

func (m *mitschnitt) Flush() {
	if f, ok := m.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Protokolliere zeichnet Methode, Pfad, alle Header, Query, Body, Antwort und
// Dauer auf. Das Token wird maskiert (mit Hinweis, ob es korrekt war).
func Protokolliere(p *Protokoll, token string, auslassen func(*http.Request) bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auslassen != nil && auslassen(r) {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		var body []byte
		if r.Body != nil {
			body, _ = io.ReadAll(io.LimitReader(r.Body, maxBodyProtokoll))
			_ = r.Body.Close()
			r.Body = io.NopCloser(bytes.NewReader(body))
		}
		ctx, n := mitNotizen(r.Context())
		r = r.WithContext(ctx)
		ms := &mitschnitt{ResponseWriter: w}
		next.ServeHTTP(ms, r)

		if r.Context().Err() != nil {
			notiz(ctx, "verbindung_vom_aufrufer_getrennt", true)
		}
		p.Hinzufuegen(Eintrag{
			Zeit:       start,
			Art:        "http",
			Methode:    r.Method,
			Pfad:       r.URL.Path,
			Query:      r.URL.Query(),
			Header:     maskiert(r.Header, token),
			Body:       alsJSONoderText(body),
			RemoteAddr: r.RemoteAddr,
			Status:     ms.status,
			Antwort:    alsJSONoderText(ms.buf.Bytes()),
			DauerMs:    time.Since(start).Milliseconds(),
			Notizen:    n.kopie(),
		})
	})
}

func alsJSONoderText(b []byte) any {
	b = bytes.TrimSpace(b)
	if len(b) == 0 {
		return nil
	}
	if json.Valid(b) {
		return json.RawMessage(append([]byte(nil), b...))
	}
	return string(b)
}

// maskiert ersetzt Token-Werte, zeigt aber, ob sie korrekt waren.
func maskiert(h http.Header, token string) map[string][]string {
	out := map[string][]string{}
	for k, vs := range h {
		kopie := append([]string(nil), vs...)
		for i, v := range kopie {
			switch http.CanonicalHeaderKey(k) {
			case "Authorization":
				roh := strings.TrimSpace(v)
				praefix := ""
				if len(roh) >= 7 && strings.EqualFold(roh[:7], "bearer ") {
					praefix = roh[:7]
					roh = strings.TrimSpace(roh[7:])
				}
				kopie[i] = praefix + maske(roh, token)
			case "X-Api-Key":
				kopie[i] = maske(strings.TrimSpace(v), token)
			}
		}
		out[k] = kopie
	}
	return out
}

func maske(wert, token string) string {
	if token != "" && tokenGleich(wert, token) {
		return "***(korrekt)"
	}
	return fmt.Sprintf("***(falsch, %d Zeichen)", len(wert))
}

// --- GET/DELETE /protokoll ---

func protokollHandler(p *Protokoll) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodDelete:
			p.Leeren()
			schreibeJSON(w, http.StatusOK, map[string]any{"geleert": true})
		case http.MethodGet:
			seit, err := parseSeit(r.URL.Query().Get("seit"), time.Now())
			if err != nil {
				schreibeJSON(w, http.StatusBadRequest, map[string]any{"fehler": err.Error()})
				return
			}
			liste := p.Eintraege(seit, r.URL.Query().Get("pfad"))
			schreibeJSON(w, http.StatusOK, map[string]any{"anzahl": len(liste), "eintraege": liste})
		default:
			w.Header().Set("Allow", "GET, DELETE")
			schreibeJSON(w, http.StatusMethodNotAllowed, map[string]any{"fehler": "nur GET oder DELETE"})
		}
	})
}

// parseSeit akzeptiert RFC3339 (2026-09-26T10:00:00+02:00) oder eine Dauer (15m, 2h).
func parseSeit(s string, jetzt time.Time) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if d, err := time.ParseDuration(s); err == nil {
		return jetzt.Add(-d), nil
	}
	return time.Time{}, fmt.Errorf("seit=%q: erwartet RFC3339 (z. B. 2026-09-26T10:00:00+02:00) oder Dauer (z. B. 15m)", s)
}

func schreibeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
