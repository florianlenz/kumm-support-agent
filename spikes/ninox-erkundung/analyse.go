package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Mask ersetzt jeden Buchstaben durch "A" und jede Ziffer durch "9".
// Alles andere (Punkte, Bindestriche, Leerzeichen, Schrägstriche …) bleibt
// stehen. So bleibt das Format einer Maschinennummer sichtbar, ohne dass
// echte Daten im Bericht landen: "XXX36.1024" → "AAA99.9999".
func Mask(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case unicode.IsLetter(r):
			b.WriteRune('A')
		case unicode.IsDigit(r):
			b.WriteRune('9')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Normalize vereinheitlicht eine Maschinennummer für den Vergleich:
// Punkte, Leerzeichen (jede Art von Leerraum) und Bindestriche fallen weg,
// Buchstaben werden groß geschrieben. "tkkc-99.01" → "TKKC9901".
func Normalize(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.IsSpace(r) || isStripped(r) {
			continue
		}
		b.WriteRune(unicode.ToUpper(r))
	}
	return b.String()
}

func isStripped(r rune) bool {
	switch r {
	case '.', '-', '‐', '‑', '‒', '–', '—', '−':
		return true
	}
	return false
}

// Suffix liefert die letzten n Zeichen (Runen, nicht Bytes) von s.
// Ist s kürzer als n, kommt s unverändert zurück.
func Suffix(s string, n int) string {
	if n <= 0 {
		return ""
	}
	count := utf8.RuneCountInString(s)
	if count <= n {
		return s
	}
	skip := count - n
	for i := range s {
		if skip == 0 {
			return s[i:]
		}
		skip--
	}
	return ""
}

// ValueText wandelt einen JSON-Feldwert in Text um, sofern er als
// Maschinennummer auswertbar ist. kind beschreibt die JSON-Art des Werts.
// ok ist false für leere Werte, Listen und Objekte.
func ValueText(v any) (text string, kind string, ok bool) {
	switch t := v.(type) {
	case nil:
		return "", "leer", false
	case string:
		if strings.TrimSpace(t) == "" {
			return "", "leer", false
		}
		return t, "Text", true
	case json.Number:
		return t.String(), "Zahl", true
	case float64:
		return fmt.Sprintf("%v", t), "Zahl", true
	case bool:
		return "", "Wahrheitswert", false
	case []any:
		return "", "Liste", false
	case map[string]any:
		return "", "Objekt", false
	default:
		return "", fmt.Sprintf("%T", v), false
	}
}

// SuffixLen ist die Anzahl Zeichen, die der Anrufer laut Map (#1) nennt:
// „die letzten 8 Zeichen der Maschinennummer“.
const SuffixLen = 8

// Count ist ein Muster mit seiner Häufigkeit.
type Count struct {
	Key string
	N   int
}

// Analysis fasst die Werte eines Feldes zusammen, ohne Rohwerte zu behalten.
type Analysis struct {
	Records   int            // gelesene Datensätze
	Kinds     map[string]int // JSON-Art → Anzahl (Text, Zahl, leer, Liste …)
	Evaluated int            // als Text auswertbare Werte

	Patterns       []Count     // maskierte Muster, häufigste zuerst
	Lengths        map[int]int // Länge (nach Vereinheitlichung) → Anzahl
	WithLowercase  int         // Werte mit Kleinbuchstaben
	WithWhitespace int         // Werte mit Leerraum
	WithDot        int
	WithDash       int
	WithOther      int // Werte mit sonstigen Sonderzeichen (/, _, # …)
	OnlyDigits     int // nach Vereinheitlichung nur Ziffern

	RawDupGroups     int // gleiche Rohwerte, Gruppen mit > 1 Datensatz
	RawDupRecords    int // Datensätze in diesen Gruppen
	NormDupGroups    int // gleiche Werte nach Vereinheitlichung
	NormDupRecords   int
	NormOnlyDupGroup int // Dubletten, die erst durch Vereinheitlichung entstehen

	Distinct            int // verschiedene vereinheitlichte Werte
	ShorterThanSuffix   int // verschiedene Werte mit weniger als 8 Zeichen
	SuffixUnique        int // verschiedene Werte, deren letzte 8 Zeichen eindeutig sind
	SuffixCollGroups    int // Endungen, die zu mehr als einem Wert passen
	SuffixCollValues    int // verschiedene Werte in diesen Gruppen
	SuffixCollMaxGroup  int // größte Gruppe
	SuffixCollPatterns  []Count
	PatternOverflow     int // Anzahl Muster, die nicht mehr aufgeführt werden
	SuffixPatternsTotal int
}

// Analyse wertet die Feldwerte aus. Sie behält keine Rohwerte, nur Zählungen
// und maskierte Muster.
func Analyse(values []any, maxPatterns int) Analysis {
	a := Analysis{Kinds: map[string]int{}, Lengths: map[int]int{}}
	patterns := map[string]int{}
	raw := map[string]int{}
	norm := map[string]int{}
	normRaw := map[string]map[string]struct{}{}

	for _, v := range values {
		a.Records++
		text, kind, ok := ValueText(v)
		a.Kinds[kind]++
		if !ok {
			continue
		}
		a.Evaluated++
		patterns[Mask(text)]++
		n := Normalize(text)
		a.Lengths[utf8.RuneCountInString(n)]++
		raw[text]++
		norm[n]++
		if normRaw[n] == nil {
			normRaw[n] = map[string]struct{}{}
		}
		normRaw[n][text] = struct{}{}

		if n != "" && strings.IndexFunc(n, func(r rune) bool { return !unicode.IsDigit(r) }) == -1 {
			a.OnlyDigits++
		}
		var lower, space, dot, dash, other bool
		for _, r := range text {
			switch {
			case unicode.IsLower(r):
				lower = true
			case unicode.IsSpace(r):
				space = true
			case r == '.':
				dot = true
			case isStripped(r):
				dash = true
			case !unicode.IsLetter(r) && !unicode.IsDigit(r):
				other = true
			}
		}
		a.WithLowercase += b2i(lower)
		a.WithWhitespace += b2i(space)
		a.WithDot += b2i(dot)
		a.WithDash += b2i(dash)
		a.WithOther += b2i(other)
	}

	for _, n := range raw {
		if n > 1 {
			a.RawDupGroups++
			a.RawDupRecords += n
		}
	}
	for key, n := range norm {
		if n > 1 {
			a.NormDupGroups++
			a.NormDupRecords += n
			if len(normRaw[key]) > 1 {
				a.NormOnlyDupGroup++
			}
		}
	}

	a.Distinct = len(norm)
	suffixes := map[string][]string{}
	for n := range norm {
		if utf8.RuneCountInString(n) < SuffixLen {
			a.ShorterThanSuffix++
		}
		s := Suffix(n, SuffixLen)
		suffixes[s] = append(suffixes[s], n)
	}
	collPatterns := map[string]int{}
	for s, group := range suffixes {
		if len(group) == 1 {
			a.SuffixUnique++
			continue
		}
		a.SuffixCollGroups++
		a.SuffixCollValues += len(group)
		if len(group) > a.SuffixCollMaxGroup {
			a.SuffixCollMaxGroup = len(group)
		}
		collPatterns[Mask(s)]++
	}

	a.Patterns = sortCounts(patterns)
	if maxPatterns > 0 && len(a.Patterns) > maxPatterns {
		a.PatternOverflow = len(a.Patterns) - maxPatterns
		a.Patterns = a.Patterns[:maxPatterns]
	}
	a.SuffixCollPatterns = sortCounts(collPatterns)
	a.SuffixPatternsTotal = len(a.SuffixCollPatterns)
	if len(a.SuffixCollPatterns) > 10 {
		a.SuffixCollPatterns = a.SuffixCollPatterns[:10]
	}
	return a
}

func sortCounts(m map[string]int) []Count {
	out := make([]Count, 0, len(m))
	for k, n := range m {
		out = append(out, Count{Key: k, N: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].N != out[j].N {
			return out[i].N > out[j].N
		}
		return out[i].Key < out[j].Key
	})
	return out
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
