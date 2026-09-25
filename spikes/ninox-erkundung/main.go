// Befehl ninox-erkundung liest das Datenmodell einer Ninox-Datenbank
// (nur lesend) und schreibt einen Markdown-Bericht. Optional wertet er die
// Formate eines Feldes (z. B. der Maschinennummer) maskiert aus.
//
// Siehe ANLEITUNG.md.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultBaseV3 = "https://api.ninox.com/v1"
	defaultBaseV4 = "https://go.ninox.com/api/v1"
)

type config struct {
	token, baseURL, api string
	teamID, dbID        string
	workspaceID, module string
	table, field        string
	max                 int
	outDir              string
	pause               time.Duration
	saveRaw             bool
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Fehler:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config{
		token:       strings.TrimSpace(os.Getenv("NINOX_TOKEN")),
		baseURL:     strings.TrimSpace(os.Getenv("NINOX_BASE_URL")),
		api:         strings.ToLower(strings.TrimSpace(os.Getenv("NINOX_API"))),
		teamID:      strings.TrimSpace(os.Getenv("NINOX_TEAM_ID")),
		dbID:        strings.TrimSpace(os.Getenv("NINOX_DB_ID")),
		workspaceID: strings.TrimSpace(os.Getenv("NINOX_WORKSPACE_ID")),
		module:      strings.TrimSpace(os.Getenv("NINOX_MODULE")),
	}
	flag.StringVar(&cfg.table, "tabelle", "", "Tabelle (ID oder Name) für die Werte-Auswertung, z. B. \"Maschinen\"")
	flag.StringVar(&cfg.field, "feld", "", "Feld (ID oder Name) für die Werte-Auswertung, z. B. \"Maschinennummer\"")
	flag.IntVar(&cfg.max, "max", 200, "höchstens so viele Datensätze auswerten (0 = alle)")
	flag.StringVar(&cfg.outDir, "ausgabe", "ausgabe", "Verzeichnis für die Berichte (ist per .gitignore ausgeschlossen)")
	flag.DurationVar(&cfg.pause, "pause", 250*time.Millisecond, "Pause vor jeder Anfrage (Ninox begrenzt parallele Aufrufe)")
	flag.BoolVar(&cfg.saveRaw, "roh", true, "rohes Schema (ohne Datensätze) als JSON im Ausgabeverzeichnis ablegen")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), `ninox-erkundung – liest das Ninox-Datenmodell (nur GET-Anfragen)

Umgebungsvariablen:
  NINOX_TOKEN         API-Schlüssel (Pflicht)
  NINOX_API           v3 (klassisch, Standard) oder v4 (Ninox 4); v4 wird
                      automatisch gewählt, wenn NINOX_WORKSPACE_ID gesetzt ist
  NINOX_BASE_URL      Standard v3: %s
                      Standard v4: %s
                      Private Cloud v3: https://<name>.ninoxdb.de/v1
  NINOX_TEAM_ID       (v3, optional) nur dieses Team
  NINOX_DB_ID         (v3, optional) nur diese Datenbank
  NINOX_WORKSPACE_ID  (v4, Pflicht für v4) Workspace-ID (12 Zeichen)
  NINOX_MODULE        (v4, optional) nur dieses Modul

Flags:
`, defaultBaseV3, defaultBaseV4)
		flag.PrintDefaults()
	}
	flag.Parse()

	if cfg.token == "" {
		return errors.New("NINOX_TOKEN ist nicht gesetzt (siehe ANLEITUNG.md)")
	}
	if (cfg.table == "") != (cfg.field == "") {
		return errors.New("-tabelle und -feld nur gemeinsam angeben")
	}
	if cfg.max < 0 {
		return errors.New("-max muss >= 0 sein")
	}
	if cfg.api == "" {
		cfg.api = "v3"
		if cfg.workspaceID != "" {
			cfg.api = "v4"
		}
	}

	var src Source
	var client *Client
	switch cfg.api {
	case "v3":
		if cfg.baseURL == "" {
			cfg.baseURL = defaultBaseV3
		}
		client = NewClient(cfg.baseURL, cfg.token, cfg.pause)
		src = &sourceV3{c: client, teamID: cfg.teamID, dbID: cfg.dbID}
	case "v4":
		if cfg.workspaceID == "" {
			return errors.New("für NINOX_API=v4 muss NINOX_WORKSPACE_ID gesetzt sein")
		}
		if cfg.baseURL == "" {
			cfg.baseURL = defaultBaseV4
		}
		client = NewClient(cfg.baseURL, cfg.token, cfg.pause)
		src = &sourceV4{c: client, workspaceID: cfg.workspaceID, module: cfg.module}
	default:
		return fmt.Errorf("NINOX_API=%q unbekannt (v3 oder v4)", cfg.api)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	started := time.Now()
	fmt.Fprintf(os.Stderr, "Lese Datenmodell über %s (%s) …\n", src.Name(), cfg.baseURL)
	containers, err := src.Discover(ctx)
	if err != nil {
		if IsStatus(err, http.StatusUnauthorized, http.StatusForbidden) {
			return fmt.Errorf("%w\n\nDer Schlüssel wurde abgelehnt. Passt er zur API-Variante? Ninox-3-Schlüssel → NINOX_API=v3, Ninox-4-Workspace-Schlüssel → NINOX_API=v4 mit NINOX_WORKSPACE_ID. Private Cloud → NINOX_BASE_URL setzen", err)
		}
		return err
	}

	if err := os.MkdirAll(cfg.outDir, 0o700); err != nil {
		return err
	}
	stamp := started.Format("20060102-150405")
	meta := ReportMeta{API: src.Name(), BaseURL: cfg.baseURL, Generated: started, Stats: &client.Stats}

	if cfg.saveRaw {
		for _, c := range containers {
			if c.Raw == nil {
				continue
			}
			p := filepath.Join(cfg.outDir, fmt.Sprintf("roh-schema-%s-%s.json", safe(c.RawTag), stamp))
			if err := writeJSON(p, c.Raw); err != nil {
				return err
			}
		}
	}

	meta.Elapsed = time.Since(started)
	schemaPath := filepath.Join(cfg.outDir, "schema-"+stamp+".md")
	if err := os.WriteFile(schemaPath, []byte(SchemaReport(meta, containers)), 0o600); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Schema-Bericht: %s (%d Datenbanken/Module, %d Tabellen)\n", schemaPath, len(containers), countTables(containers))

	if cfg.table == "" {
		fmt.Fprintln(os.Stderr, "Tipp: Mit -tabelle und -feld die Formate der Maschinennummer auswerten (siehe ANLEITUNG.md).")
		return nil
	}

	label, table, field, err := findField(containers, cfg.table, cfg.field)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Lese Werte von %s.%s (max %d, 0 = alle) …\n", table.Name, field.Name, cfg.max)
	sampleStart := time.Now()
	client.Stats = Stats{}
	values, err := src.Values(ctx, table, field, cfg.max)
	if err != nil {
		return fmt.Errorf("Datensätze lesen (nach %d Werten): %w", len(values), err)
	}
	a := Analyse(values, 40)
	values = nil // Rohwerte nicht länger als nötig halten
	meta.Elapsed = time.Since(sampleStart)
	samplePath := filepath.Join(cfg.outDir, "werte-"+stamp+".md")
	if err := os.WriteFile(samplePath, []byte(SampleReport(meta, label, table, field, cfg.max, a)), 0o600); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Werte-Bericht: %s (%d Datensätze, %d Muster)\n", samplePath, a.Records, len(a.Patterns)+a.PatternOverflow)
	return nil
}

// findField sucht Tabelle und Feld per ID oder Name (Groß-/Kleinschreibung egal).
func findField(cs []Container, tableQ, fieldQ string) (string, Table, Field, error) {
	type hit struct {
		label string
		t     Table
	}
	var hits []hit
	for _, c := range cs {
		for _, t := range c.Tables {
			if t.ID == tableQ || strings.EqualFold(t.Name, tableQ) {
				hits = append(hits, hit{c.Label, t})
			}
		}
	}
	switch len(hits) {
	case 0:
		return "", Table{}, Field{}, fmt.Errorf("Tabelle %q nicht gefunden – Namen stehen im Schema-Bericht", tableQ)
	case 1:
	default:
		var ls []string
		for _, h := range hits {
			ls = append(ls, h.label)
		}
		return "", Table{}, Field{}, fmt.Errorf("Tabelle %q ist mehrdeutig (%s) – NINOX_DB_ID bzw. NINOX_MODULE setzen", tableQ, strings.Join(ls, "; "))
	}
	h := hits[0]
	for _, f := range h.t.Fields {
		if f.ID == fieldQ || strings.EqualFold(f.Name, fieldQ) {
			return h.label, h.t, f, nil
		}
	}
	return "", Table{}, Field{}, fmt.Errorf("Feld %q in Tabelle %q nicht gefunden", fieldQ, h.t.Name)
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func safe(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, s)
}
