package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Foto-Status: simuliert das Warten auf ein Foto vom Typenschild per WhatsApp.
// ---------------------------------------------------------------------------

type FotoStatusIn struct {
	Anrufer    string `json:"anrufer,omitempty" jsonschema:"Rufnummer des Anrufers (wird bei API-Anfragen als fester Parameter gesetzt)"`
	Rufnummer  string `json:"rufnummer,omitempty" jsonschema:"Rufnummer des Anrufers, falls bekannt"`
	AnliegenID string `json:"anliegen_id,omitempty" jsonschema:"Die anliegen_id aus der Begrüßung, falls bekannt"`
}

type FotoStatusOut struct {
	Status          string `json:"status"`
	Hinweis         string `json:"hinweis,omitempty"`
	Maschinennummer string `json:"maschinennummer,omitempty"`
	Kunde           string `json:"kunde,omitempty"`
}

type fotoZaehler struct {
	anzahl  int
	zuletzt time.Time
}

// FotoSpeicher zählt Abfragen pro Schlüssel (Rufnummer / anliegen_id / Standard).
type FotoSpeicher struct {
	mu       sync.Mutex
	versuche int           // so viele Abfragen antworten "noch_nicht_da"
	leerlauf time.Duration // nach so langer Ruhe beginnt der Zähler neu
	jetzt    func() time.Time
	zaehler  map[string]*fotoZaehler
}

func NeuerFotoSpeicher(versuche int, leerlauf time.Duration) *FotoSpeicher {
	return &FotoSpeicher{versuche: versuche, leerlauf: leerlauf, jetzt: time.Now, zaehler: map[string]*fotoZaehler{}}
}

// istPlatzhalter erkennt nicht ersetzte Variablen wie "%%caller_number%%" oder "{{caller_number}}".
func istPlatzhalter(s string) bool {
	return strings.Contains(s, "%%") || strings.Contains(s, "{{")
}

// FotoSchluessel wählt den Zählerschlüssel: fester Parameter "anrufer" (wenn
// ersetzt), sonst "rufnummer", sonst "anliegen_id", sonst "standard".
func FotoSchluessel(in FotoStatusIn) string {
	if a := strings.TrimSpace(in.Anrufer); a != "" && !istPlatzhalter(a) {
		return "tel:" + normalisiereRufnummer(a)
	}
	if a := strings.TrimSpace(in.Rufnummer); a != "" && !istPlatzhalter(a) {
		return "tel:" + normalisiereRufnummer(a)
	}
	if a := strings.TrimSpace(in.AnliegenID); a != "" && !istPlatzhalter(a) {
		return "anliegen:" + a
	}
	return "standard"
}

func normalisiereRufnummer(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= '0' && r <= '9') || r == '+' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return s
	}
	return b.String()
}

// Abfragen zählt eine Abfrage und liefert Antwort, Schlüssel und laufende Nummer.
func (f *FotoSpeicher) Abfragen(in FotoStatusIn) (FotoStatusOut, string, int) {
	schluessel := FotoSchluessel(in)
	f.mu.Lock()
	jetzt := f.jetzt()
	z, ok := f.zaehler[schluessel]
	if !ok || jetzt.Sub(z.zuletzt) > f.leerlauf {
		z = &fotoZaehler{}
		f.zaehler[schluessel] = z
	}
	z.anzahl++
	z.zuletzt = jetzt
	nr := z.anzahl
	f.mu.Unlock()

	if nr <= f.versuche {
		return FotoStatusOut{Status: "noch_nicht_da", Hinweis: "Bitte in einigen Sekunden erneut nachfragen."}, schluessel, nr
	}
	return FotoStatusOut{Status: "erkannt", Maschinennummer: "TKKC9901", Kunde: "Testbetrieb Müller, Musterdorf"}, schluessel, nr
}

// Zuruecksetzen löscht einen Zähler (oder alle, wenn schluessel leer ist).
func (f *FotoSpeicher) Zuruecksetzen(schluessel string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if schluessel == "" {
		f.zaehler = map[string]*fotoZaehler{}
		return
	}
	delete(f.zaehler, schluessel)
}

// ---------------------------------------------------------------------------
// Maschine suchen: Dummy-Suche über die letzten Zeichen der Maschinennummer.
// ---------------------------------------------------------------------------

type MaschineSuchenIn struct {
	Maschinennummer string `json:"maschinennummer" jsonschema:"die letzten 8 Zeichen der Maschinennummer vom Typenschild"`
	AnliegenID      string `json:"anliegen_id,omitempty" jsonschema:"Die anliegen_id aus der Begrüßung, falls bekannt"`
}

type Maschine struct {
	Maschinennummer string `json:"maschinennummer"`
	Typ             string `json:"typ"`
	Kunde           string `json:"kunde"`
}

type MaschineSuchenOut struct {
	Ergebnis string     `json:"ergebnis"` // gefunden | mehrdeutig | nicht_gefunden | zu_kurz
	Gesucht  string     `json:"gesucht"`
	Maschine []Maschine `json:"maschinen,omitempty"`
	Hinweis  string     `json:"hinweis,omitempty"`
}

var testMaschinen = []Maschine{
	{Maschinennummer: "TKKC9901", Typ: "Güllefass 18 m³", Kunde: "Testbetrieb Müller, Musterdorf"},
	{Maschinennummer: "XXX36.1024", Typ: "Güllefass 12 m³", Kunde: "Hof Schneider, Beispielhausen"},
	{Maschinennummer: "WK-2019-00457", Typ: "Pumptankwagen 21 m³", Kunde: "Agrar GbR Weber, Probstadt"},
	// Mehrdeutig: beide enden auf 12345678.
	{Maschinennummer: "AB12345678", Typ: "Güllefass 15 m³", Kunde: "Lohnunternehmen Fischer, Testingen"},
	{Maschinennummer: "CD 1234 5678", Typ: "Güllefass 24 m³", Kunde: "Milchhof Wagner, Demodorf"},
}

// NormalisiereMaschinennummer entfernt Punkte, Leerzeichen, Binde- und
// Schrägstriche und schreibt groß.
func NormalisiereMaschinennummer(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		switch r {
		case '.', ' ', '-', '_', '/', '\t', '–':
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

const minSuchLaenge = 4

func MaschineSuchen(in MaschineSuchenIn) MaschineSuchenOut {
	gesucht := NormalisiereMaschinennummer(in.Maschinennummer)
	if len([]rune(gesucht)) < minSuchLaenge {
		return MaschineSuchenOut{Ergebnis: "zu_kurz", Gesucht: gesucht, Hinweis: "Bitte mindestens die letzten 4, besser 8 Zeichen der Maschinennummer nennen."}
	}
	var treffer []Maschine
	for _, m := range testMaschinen {
		if strings.HasSuffix(NormalisiereMaschinennummer(m.Maschinennummer), gesucht) {
			treffer = append(treffer, m)
		}
	}
	switch len(treffer) {
	case 0:
		return MaschineSuchenOut{Ergebnis: "nicht_gefunden", Gesucht: gesucht, Hinweis: "Keine Maschine gefunden. Bitte Nummer noch einmal buchstabieren lassen."}
	case 1:
		return MaschineSuchenOut{Ergebnis: "gefunden", Gesucht: gesucht, Maschine: treffer}
	default:
		return MaschineSuchenOut{Ergebnis: "mehrdeutig", Gesucht: gesucht, Maschine: treffer, Hinweis: "Mehrere Maschinen passen. Bitte nach dem Namen des Betriebs fragen."}
	}
}

// ---------------------------------------------------------------------------
// Verzögert: antwortet erst nach n Sekunden (misst, wie lange Placetel wartet).
// ---------------------------------------------------------------------------

type VerzoegertIn struct {
	Sekunden   int    `json:"sekunden" jsonschema:"Wartezeit in Sekunden (5, 15 oder 25)"`
	AnliegenID string `json:"anliegen_id,omitempty" jsonschema:"Die anliegen_id aus der Begrüßung, falls bekannt"`
}

type VerzoegertOut struct {
	Ergebnis          string `json:"ergebnis"`
	Kennwort          string `json:"kennwort"`
	WartezeitSekunden int    `json:"wartezeit_sekunden"`
	Empfangen         string `json:"empfangen_um"`
	Geantwortet       string `json:"geantwortet_um"`
}

const maxVerzoegerung = 120

// Kennwort ist ein leicht auszusprechendes Wort je Wartezeit. Nennt der
// Telefonassistent es, ist die Antwort nachweislich angekommen.
func Kennwort(sekunden int) string {
	switch sekunden {
	case 5:
		return "Apfel"
	case 15:
		return "Birne"
	case 25:
		return "Kirsche"
	case 30:
		return "Pflaume"
	default:
		return fmt.Sprintf("Zahl %d", sekunden)
	}
}

// Verzoegert wartet und gibt den Fehler des Kontexts zurück, falls der
// Aufrufer vorher auflegt/abbricht.
func Verzoegert(ctx context.Context, sekunden int) (VerzoegertOut, error) {
	if sekunden < 0 || sekunden > maxVerzoegerung {
		return VerzoegertOut{}, fmt.Errorf("sekunden muss zwischen 0 und %d liegen", maxVerzoegerung)
	}
	start := time.Now()
	t := time.NewTimer(time.Duration(sekunden) * time.Second)
	defer t.Stop()
	select {
	case <-ctx.Done():
		notiz(ctx, "abgebrochen_nach_ms", time.Since(start).Milliseconds())
		return VerzoegertOut{}, ctx.Err()
	case <-t.C:
	}
	return VerzoegertOut{
		Ergebnis:          "fertig",
		Kennwort:          Kennwort(sekunden),
		WartezeitSekunden: sekunden,
		Empfangen:         start.Format(time.RFC3339),
		Geantwortet:       time.Now().Format(time.RFC3339),
	}, nil
}

// ---------------------------------------------------------------------------
// Anliegen-IDs aus dem Inbound Webhook (für Prüfung 6).
// ---------------------------------------------------------------------------

type AnliegenRegister struct {
	mu  sync.Mutex
	ids map[string]anliegenInfo
}

type anliegenInfo struct {
	Anrufer string    `json:"anrufer"`
	Zeit    time.Time `json:"zeit"`
}

func NeuesAnliegenRegister() *AnliegenRegister {
	return &AnliegenRegister{ids: map[string]anliegenInfo{}}
}

// Neu erzeugt eine 4-stellige anliegen_id (leicht vorzulesen).
func (a *AnliegenRegister) Neu(anrufer string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	for {
		n, _ := rand.Int(rand.Reader, big.NewInt(9000))
		id := fmt.Sprintf("%d", 1000+n.Int64())
		if _, vergeben := a.ids[id]; !vergeben || len(a.ids) >= 9000 {
			a.ids[id] = anliegenInfo{Anrufer: anrufer, Zeit: time.Now()}
			return id
		}
	}
}

// Pruefen sagt, ob eine an ein Werkzeug übergebene anliegen_id vom Inbound Webhook stammt.
func (a *AnliegenRegister) Pruefen(id string) (anliegenInfo, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	info, ok := a.ids[strings.TrimSpace(id)]
	return info, ok
}

// anliegenNotiz vermerkt im Protokoll, ob eine übergebene anliegen_id bekannt ist.
func anliegenNotiz(ctx context.Context, reg *AnliegenRegister, id string) {
	if strings.TrimSpace(id) == "" {
		notiz(ctx, "anliegen_id", "nicht übergeben")
		return
	}
	if info, ok := reg.Pruefen(id); ok {
		notiz(ctx, "anliegen_id", fmt.Sprintf("%s: bekannt aus Inbound Webhook (Anrufer %q, %s)", id, info.Anrufer, info.Zeit.Format(time.RFC3339)))
		return
	}
	notiz(ctx, "anliegen_id", fmt.Sprintf("%s: UNBEKANNT (nicht vom Inbound Webhook vergeben)", id))
}
