package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMask(t *testing.T) {
	cases := map[string]string{
		"TKKC9901":   "AAAA9999",
		"XXX36.1024": "AAA99.9999",
		"12345678":   "99999999",
		"ab-12 / Ü7": "AA-99 / A9",
		"":           "",
		"Nr.#1_x":    "AA.#9_A",
		"ÄÖÜß ٣":     "AAAA 9", // Unicode-Buchstaben und -Ziffern
	}
	for in, want := range cases {
		if got := Mask(in); got != want {
			t.Errorf("Mask(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"TKKC9901":      "TKKC9901",
		"tkkc-99.01":    "TKKC9901",
		" XXX 36.1024 ": "XXX361024",
		"a\tb c":        "ABC",     // Tab und geschütztes Leerzeichen
		"12–34":         "1234",    // Halbgeviertstrich
		"AB/12_3":       "AB/12_3", // andere Zeichen bleiben
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSuffix(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"XXX361024", 8, "XX361024"},
		{"1234", 8, "1234"},
		{"ÄÄÄÄÄÄÄÄÄ", 8, "ÄÄÄÄÄÄÄÄ"},
		{"abc", 0, ""},
	}
	for _, c := range cases {
		if got := Suffix(c.in, c.n); got != c.want {
			t.Errorf("Suffix(%q, %d) = %q, want %q", c.in, c.n, got, c.want)
		}
	}
}

func TestValueText(t *testing.T) {
	if s, k, ok := ValueText(json.Number("12345678")); !ok || s != "12345678" || k != "Zahl" {
		t.Errorf("json.Number: %q %q %v", s, k, ok)
	}
	for _, v := range []any{nil, "  ", []any{1}, map[string]any{}, true} {
		if _, _, ok := ValueText(v); ok {
			t.Errorf("ValueText(%#v) sollte nicht auswertbar sein", v)
		}
	}
}

func TestAnalyse(t *testing.T) {
	values := []any{
		"TKKC9901",
		"tkkc-9901", // Dublette erst nach Vereinheitlichung
		"XXX36.1024",
		"XXX36.1024", // Rohdublette
		"YXX36.1024", // gleiche letzte 8 Zeichen wie XXX361024 → Kollision
		json.Number("12345678"),
		"123",
		nil,
		"",
		[]any{json.Number("4")},
	}
	a := Analyse(values, 0)

	check := func(name string, got, want int) {
		t.Helper()
		if got != want {
			t.Errorf("%s = %d, want %d", name, got, want)
		}
	}
	check("Records", a.Records, 10)
	check("Evaluated", a.Evaluated, 7)
	check("Kinds[leer]", a.Kinds["leer"], 2)
	check("Kinds[Liste]", a.Kinds["Liste"], 1)
	check("Kinds[Zahl]", a.Kinds["Zahl"], 1)
	check("RawDupGroups", a.RawDupGroups, 1)
	check("RawDupRecords", a.RawDupRecords, 2)
	check("NormDupGroups", a.NormDupGroups, 2)
	check("NormDupRecords", a.NormDupRecords, 4)
	check("NormOnlyDupGroup", a.NormOnlyDupGroup, 1)
	check("Distinct", a.Distinct, 5) // TKKC9901, XXX361024, YXX361024, 12345678, 123
	check("ShorterThanSuffix", a.ShorterThanSuffix, 1)
	check("SuffixCollGroups", a.SuffixCollGroups, 1)
	check("SuffixCollValues", a.SuffixCollValues, 2)
	check("SuffixUnique", a.SuffixUnique, 3)
	check("OnlyDigits", a.OnlyDigits, 2)
	check("WithLowercase", a.WithLowercase, 1)
	check("WithDot", a.WithDot, 3)
	check("WithDash", a.WithDash, 1)

	if len(a.Patterns) == 0 || a.Patterns[0].Key != "AAA99.9999" || a.Patterns[0].N != 3 {
		t.Errorf("häufigstes Muster = %+v, want AAA99.9999×3", a.Patterns)
	}
	if len(a.SuffixCollPatterns) != 1 || a.SuffixCollPatterns[0].Key != "AA999999" {
		t.Errorf("SuffixCollPatterns = %+v", a.SuffixCollPatterns)
	}
}

// Der Werte-Bericht darf keine Rohwerte enthalten.
func TestSampleReportLeaksNoValues(t *testing.T) {
	secret := []string{"TKKC9901", "XXX36.1024", "Hofmann"}
	var values []any
	for _, s := range secret {
		values = append(values, s, s)
	}
	a := Analyse(values, 40)
	out := SampleReport(ReportMeta{Stats: &Stats{}}, "Test", Table{ID: "A", Name: "Maschinen"},
		Field{ID: "B", Name: "Maschinennummer", Type: "string"}, 0, a)
	for _, s := range secret {
		if strings.Contains(out, s) || strings.Contains(out, Normalize(s)) || strings.Contains(out, Suffix(Normalize(s), SuffixLen)) {
			t.Errorf("Bericht enthält Rohwert %q", s)
		}
	}
	if !strings.Contains(out, "AAAA9999") {
		t.Errorf("Bericht enthält das maskierte Muster nicht:\n%s", out)
	}
}

func TestAnalysePatternLimit(t *testing.T) {
	a := Analyse([]any{"A", "1", "A1", "1A"}, 2)
	if len(a.Patterns) != 2 || a.PatternOverflow != 2 {
		t.Errorf("Patterns=%d Overflow=%d", len(a.Patterns), a.PatternOverflow)
	}
}
