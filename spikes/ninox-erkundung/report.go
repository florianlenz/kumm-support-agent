package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Hinweise auf Kandidaten für Maschine/Kunde (nur Namen, keine Daten).
var (
	machineHint  = regexp.MustCompile(`(?i)masch|fahrgestell|serien|fg-?nr|fz-?nr|typenschild|gerät|geraet|fass`)
	customerHint = regexp.MustCompile(`(?i)kunde|kunden|betrieb|firma|landwirt|adresse|ort|plz|name|kontakt|telefon|mail`)
)

type ReportMeta struct {
	API       string
	BaseURL   string
	Generated time.Time
	Stats     *Stats
	Elapsed   time.Duration
}

// SchemaReport erzeugt den Markdown-Bericht über das Datenmodell.
func SchemaReport(meta ReportMeta, containers []Container) string {
	var b strings.Builder
	w := func(format string, args ...any) { fmt.Fprintf(&b, format, args...) }

	w("# Ninox-Erkundung: Datenmodell\n\n")
	w("- Erzeugt: %s\n", meta.Generated.Format("2006-01-02 15:04 MST"))
	w("- API: %s\n", meta.API)
	w("- Basis-URL: `%s`\n", meta.BaseURL)
	w("- Datenbanken/Module: %d, Tabellen: %d\n\n", len(containers), countTables(containers))
	w("Dieser Bericht enthält nur Strukturangaben (Tabellen, Felder, Typen), keine Datensätze.\n\n")

	// Überblick Verknüpfungen
	w("## Verknüpfungen\n\n")
	found := false
	for _, c := range containers {
		for _, t := range c.Tables {
			for _, f := range t.Fields {
				if f.RefTable != "" || isRefType(f.Type) {
					if !found {
						w("| Datenbank/Modul | Tabelle | Feld | Typ | Ziel |\n|---|---|---|---|---|\n")
						found = true
					}
					w("| %s | %s | %s | %s | %s |\n", md(shortLabel(c.Label)), md(t.Name), md(f.Name), md(f.Type), md(orDash(f.RefTable)))
				}
			}
		}
	}
	if !found {
		w("_Keine Verknüpfungsfelder erkannt._\n")
	}
	w("\n")

	// Kandidaten
	w("## Kandidaten (nach Namen geraten)\n\n")
	w("Felder, deren Name auf Maschinennummer bzw. Kundendaten hindeuten könnte:\n\n")
	for _, c := range containers {
		for _, t := range c.Tables {
			var m, k []string
			for _, f := range t.Fields {
				label := fmt.Sprintf("`%s` (%s)", f.Name, f.Type)
				if machineHint.MatchString(f.Name) {
					m = append(m, label)
				} else if customerHint.MatchString(f.Name) {
					k = append(k, label)
				}
			}
			if len(m)+len(k) == 0 && !machineHint.MatchString(t.Name) && !customerHint.MatchString(t.Name) {
				continue
			}
			w("- **%s** (%s)", md(t.Name), md(t.ID))
			if len(m) > 0 {
				w(" – Maschine?: %s", strings.Join(m, ", "))
			}
			if len(k) > 0 {
				w(" – Kunde?: %s", strings.Join(k, ", "))
			}
			w("\n")
		}
	}
	w("\n")

	// Details
	w("## Tabellen und Felder\n\n")
	for _, c := range containers {
		w("### %s\n\n", md(c.Label))
		for _, n := range c.Notes {
			w("> Hinweis: %s\n\n", md(n))
		}
		for _, t := range c.Tables {
			w("#### Tabelle „%s“ (ID `%s`, %d Felder)\n\n", md(t.Name), t.ID, len(t.Fields))
			w("| ID | Name | Typ | Verweis auf | Details |\n|---|---|---|---|---|\n")
			for _, f := range t.Fields {
				w("| `%s` | %s | %s | %s | %s |\n", f.ID, md(f.Name), md(f.Type), md(orDash(f.RefTable)), md(details(f.Details)))
			}
			w("\n")
		}
	}

	w("## API-Verhalten\n\n")
	writeStats(&b, meta)
	return b.String()
}

func writeStats(b *strings.Builder, meta ReportMeta) {
	s := meta.Stats
	fmt.Fprintf(b, "- Anfragen: %d (Wiederholungen: %d), Gesamtdauer: %s\n", len(s.Durations), s.Retries, meta.Elapsed.Round(time.Millisecond))
	if len(s.Durations) > 0 {
		fmt.Fprintf(b, "- Antwortzeit: min %s · Median %s · p95 %s · max %s\n",
			s.Percentile(0).Round(time.Millisecond), s.Percentile(0.5).Round(time.Millisecond),
			s.Percentile(0.95).Round(time.Millisecond), s.Percentile(1).Round(time.Millisecond))
	}
	codes := make([]int, 0, len(s.Status))
	for c := range s.Status {
		codes = append(codes, c)
	}
	sort.Ints(codes)
	var parts []string
	for _, c := range codes {
		parts = append(parts, fmt.Sprintf("%d×%d", s.Status[c], c))
	}
	fmt.Fprintf(b, "- Statuscodes: %s\n", orDash(strings.Join(parts, ", ")))
	if s.Status[429] > 0 {
		fmt.Fprintf(b, "- **Rate-Limit erreicht** (HTTP 429) – Pause mit `-pause` erhöhen.\n")
	}
	fmt.Fprintf(b, "\n")
}

// SampleReport beschreibt die Werte eines Feldes – nur maskiert und gezählt.
func SampleReport(meta ReportMeta, containerLabel string, t Table, f Field, max int, a Analysis) string {
	var b strings.Builder
	w := func(format string, args ...any) { fmt.Fprintf(&b, format, args...) }
	pct := func(n int) string {
		if a.Evaluated == 0 {
			return "–"
		}
		return fmt.Sprintf("%.1f %%", 100*float64(n)/float64(a.Evaluated))
	}

	w("# Ninox-Erkundung: Werteformate\n\n")
	w("- Erzeugt: %s\n", meta.Generated.Format("2006-01-02 15:04 MST"))
	w("- Quelle: %s › Tabelle „%s“ (`%s`) › Feld „%s“ (`%s`, Typ %s)\n", md(containerLabel), md(t.Name), t.ID, md(f.Name), f.ID, md(f.Type))
	if max > 0 {
		w("- Stichprobe: höchstens %d Datensätze (die ersten laut API-Reihenfolge)\n", max)
	} else {
		w("- Stichprobe: alle Datensätze\n")
	}
	w("\nAlle Werte sind maskiert: Buchstabe → `A`, Ziffer → `9`, übrige Zeichen bleiben. Es stehen keine echten Werte in diesem Bericht.\n\n")

	w("## Mengen\n\n")
	w("- Gelesene Datensätze: **%d**\n", a.Records)
	w("- Auswertbare Werte: %d\n", a.Evaluated)
	w("- Wertarten: %s\n", kinds(a.Kinds))
	w("- Verschiedene Werte nach Vereinheitlichung: %d\n\n", a.Distinct)

	w("## Formate (maskiert)\n\n")
	w("| Muster | Anzahl |\n|---|---|\n")
	for _, p := range a.Patterns {
		w("| `%s` | %d |\n", strings.ReplaceAll(p.Key, "`", "'"), p.N)
	}
	if a.PatternOverflow > 0 {
		w("| … %d weitere Muster | |\n", a.PatternOverflow)
	}
	w("\nZeichen in den Werten:\n\n")
	w("- mit Kleinbuchstaben: %d (%s)\n", a.WithLowercase, pct(a.WithLowercase))
	w("- mit Leerzeichen: %d (%s)\n", a.WithWhitespace, pct(a.WithWhitespace))
	w("- mit Punkt: %d (%s)\n", a.WithDot, pct(a.WithDot))
	w("- mit Bindestrich: %d (%s)\n", a.WithDash, pct(a.WithDash))
	w("- mit sonstigen Sonderzeichen: %d (%s)\n", a.WithOther, pct(a.WithOther))
	w("- nur Ziffern (nach Vereinheitlichung): %d (%s)\n\n", a.OnlyDigits, pct(a.OnlyDigits))

	w("Längen nach Vereinheitlichung (Zeichen → Anzahl): %s\n\n", lengths(a.Lengths))

	w("## Dubletten\n\n")
	w("Vereinheitlichung: Punkte, Leerzeichen und Bindestriche entfernen, Großschreibung.\n\n")
	w("- Gleiche Rohwerte: %d Gruppen, %d Datensätze\n", a.RawDupGroups, a.RawDupRecords)
	w("- Gleiche Werte nach Vereinheitlichung: %d Gruppen, %d Datensätze\n", a.NormDupGroups, a.NormDupRecords)
	w("- davon erst durch Vereinheitlichung gleich: %d Gruppen\n\n", a.NormOnlyDupGroup)

	w("## Letzte %d Zeichen\n\n", SuffixLen)
	w("Basis: %d verschiedene vereinheitlichte Werte.\n\n", a.Distinct)
	w("- Werte kürzer als %d Zeichen: %d\n", SuffixLen, a.ShorterThanSuffix)
	w("- Werte mit eindeutiger Endung: %d", a.SuffixUnique)
	if a.Distinct > 0 {
		w(" (%.1f %%)", 100*float64(a.SuffixUnique)/float64(a.Distinct))
	}
	w("\n- Mehrdeutige Endungen: %d, betroffen: %d Werte, größte Gruppe: %d\n", a.SuffixCollGroups, a.SuffixCollValues, a.SuffixCollMaxGroup)
	if len(a.SuffixCollPatterns) > 0 {
		w("- Muster der mehrdeutigen Endungen (maskiert): ")
		var parts []string
		for _, p := range a.SuffixCollPatterns {
			parts = append(parts, fmt.Sprintf("`%s`×%d", p.Key, p.N))
		}
		w("%s", strings.Join(parts, ", "))
		if a.SuffixPatternsTotal > len(a.SuffixCollPatterns) {
			w(", …")
		}
		w("\n")
	}
	w("\n## API-Verhalten\n\n")
	writeStats(&b, meta)
	return b.String()
}

func details(d map[string]any) string {
	var parts []string
	for _, k := range sortedKeys(d) {
		v := d[k]
		var s string
		switch t := v.(type) {
		case nil:
			continue
		case string:
			if t == "" {
				continue
			}
			if len([]rune(t)) > 40 {
				s = fmt.Sprintf("Text/Formel (%d Zeichen)", len([]rune(t)))
			} else {
				s = t
			}
		case []any:
			s = fmt.Sprintf("[%d]", len(t))
		case map[string]any:
			s = fmt.Sprintf("{%d}", len(t))
		case bool, json.Number, float64:
			s = fmt.Sprint(t)
		default:
			s = fmt.Sprint(t)
		}
		parts = append(parts, k+"="+s)
	}
	return orDash(strings.Join(parts, "; "))
}

func isRefType(t string) bool {
	switch strings.ToLower(t) {
	case "ref", "reference", "rev", "reverse", "link":
		return true
	}
	return false
}

func kinds(m map[string]int) string {
	var parts []string
	for _, k := range sortedKeys(m) {
		parts = append(parts, fmt.Sprintf("%s %d", k, m[k]))
	}
	return orDash(strings.Join(parts, ", "))
}

func lengths(m map[int]int) string {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%d→%d", k, m[k]))
	}
	return orDash(strings.Join(parts, ", "))
}

func countTables(cs []Container) int {
	n := 0
	for _, c := range cs {
		n += len(c.Tables)
	}
	return n
}

func shortLabel(l string) string {
	if i := strings.LastIndex(l, "›"); i >= 0 {
		return strings.TrimSpace(l[i+len("›"):])
	}
	return l
}

func orDash(s string) string {
	if s == "" {
		return "–"
	}
	return s
}

// md entschärft Text für Markdown-Tabellen.
func md(s string) string {
	s = strings.ReplaceAll(s, "|", `\|`)
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}
