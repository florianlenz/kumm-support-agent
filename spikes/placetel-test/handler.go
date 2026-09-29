package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

type Dienst struct {
	Foto     *FotoSpeicher
	Anliegen *AnliegenRegister
}

// leseJSON dekodiert den Body tolerant: leerer Body ist erlaubt, unbekannte Felder auch.
func leseJSON(r *http.Request, ziel any) error {
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return nil
	}
	return json.Unmarshal(b, ziel)
}

// POST /api/foto-status
func (d *Dienst) fotoStatus(w http.ResponseWriter, r *http.Request) {
	var in FotoStatusIn
	if err := leseJSON(r, &in); err != nil {
		schreibeJSON(w, http.StatusBadRequest, map[string]any{"fehler": "ungültiges JSON: " + err.Error()})
		return
	}
	ctx := r.Context()
	switch {
	case in.Anrufer == "":
		notiz(ctx, "fester_parameter_anrufer", "fehlt")
	case istPlatzhalter(in.Anrufer):
		notiz(ctx, "fester_parameter_anrufer", "NICHT ersetzt: "+in.Anrufer)
	default:
		notiz(ctx, "fester_parameter_anrufer", "ersetzt: "+in.Anrufer)
	}
	anliegenNotiz(ctx, d.Anliegen, in.AnliegenID)
	out, schluessel, nr := d.Foto.Abfragen(in)
	notiz(ctx, "foto_schluessel", schluessel)
	notiz(ctx, "foto_abfrage_nr", nr)
	schreibeJSON(w, http.StatusOK, out)
}

// POST /api/foto-status/reset  Body optional: {"schluessel":"tel:+49..."}
func (d *Dienst) fotoReset(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Schluessel string `json:"schluessel"`
	}
	if err := leseJSON(r, &in); err != nil {
		schreibeJSON(w, http.StatusBadRequest, map[string]any{"fehler": "ungültiges JSON: " + err.Error()})
		return
	}
	d.Foto.Zuruecksetzen(in.Schluessel)
	alle := in.Schluessel == ""
	schreibeJSON(w, http.StatusOK, map[string]any{"zurueckgesetzt": true, "alle": alle, "schluessel": in.Schluessel})
}

// POST /api/verzoegert/{sekunden}
func (d *Dienst) verzoegert(w http.ResponseWriter, r *http.Request) {
	sek, err := strconv.Atoi(r.PathValue("sekunden"))
	if err != nil {
		schreibeJSON(w, http.StatusBadRequest, map[string]any{"fehler": "sekunden muss eine Zahl sein"})
		return
	}
	var in VerzoegertIn
	_ = leseJSON(r, &in) // Body ist optional
	anliegenNotiz(r.Context(), d.Anliegen, in.AnliegenID)
	out, err := Verzoegert(r.Context(), sek)
	if err != nil {
		if errors.Is(err, r.Context().Err()) && r.Context().Err() != nil {
			return // Aufrufer hat abgebrochen; Notiz steht im Protokoll
		}
		schreibeJSON(w, http.StatusBadRequest, map[string]any{"fehler": err.Error()})
		return
	}
	schreibeJSON(w, http.StatusOK, out)
}

// POST /api/maschine-suchen
func (d *Dienst) maschineSuchen(w http.ResponseWriter, r *http.Request) {
	var in MaschineSuchenIn
	if err := leseJSON(r, &in); err != nil {
		schreibeJSON(w, http.StatusBadRequest, map[string]any{"fehler": "ungültiges JSON: " + err.Error()})
		return
	}
	anliegenNotiz(r.Context(), d.Anliegen, in.AnliegenID)
	schreibeJSON(w, http.StatusOK, MaschineSuchen(in))
}

// POST /webhook/inbound — wird von Placetel vor Annahme des Anrufs aufgerufen.
// Body frei; erwartet z. B. {"caller_number":"{{caller_number}}", ...}.
func (d *Dienst) inbound(w http.ResponseWriter, r *http.Request) {
	var in map[string]any
	_ = leseJSON(r, &in)
	anrufer, _ := in["caller_number"].(string)
	if anrufer == "" {
		anrufer = r.URL.Query().Get("caller_number")
	}
	id := d.Anliegen.Neu(anrufer)
	notiz(r.Context(), "vergebene_anliegen_id", id)
	schreibeJSON(w, http.StatusOK, map[string]any{
		"anliegen_id":         id,
		"begruessung_hinweis": "Ihre Anliegen-Nummer lautet " + id + ".",
	})
}

var conversationIDAusLink = regexp.MustCompile(`conversation_id=([^&#\s"]+)`)

// POST /webhook/nachbearbeitung — API-Aufgabe der Nachbearbeitung nach dem Anruf.
func (d *Dienst) nachbearbeitung(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	ctx := r.Context()
	var in map[string]any
	if err := json.Unmarshal(b, &in); err != nil {
		notiz(ctx, "json_gueltig", "NEIN: "+err.Error()+" (evtl. Anführungszeichen/Zeilenumbrüche im Transkript nicht maskiert)")
	} else {
		notiz(ctx, "json_gueltig", "ja")
	}
	text := string(b)

	// Prüfung 5: conversationId gefüllt und passend zum conversation_link?
	convID, _ := in["conversationId"].(string)
	link, _ := in["conversation_link"].(string)
	switch {
	case convID == "":
		notiz(ctx, "pruefung5_conversationId", "leer oder fehlt")
	case istPlatzhalter(convID):
		notiz(ctx, "pruefung5_conversationId", "NICHT ersetzt: "+convID)
	default:
		if m := conversationIDAusLink.FindStringSubmatch(link); m != nil {
			if m[1] == convID {
				notiz(ctx, "pruefung5_conversationId", "gefüllt und GLEICH wie im conversation_link")
			} else {
				notiz(ctx, "pruefung5_conversationId", "gefüllt, aber ANDERS als im conversation_link ("+m[1]+")")
			}
		} else {
			notiz(ctx, "pruefung5_conversationId", "gefüllt; conversation_link enthält kein conversation_id=")
		}
	}

	// Prüfung 4: Stehen Antworten unserer Werkzeuge im Transkript?
	transcript, _ := in["transcript"].(string)
	if transcript == "" {
		transcript = text
	}
	gefunden := []string{}
	for _, merkmal := range []string{"TKKC9901", "noch_nicht_da", "erkannt", "Apfel", "Birne", "Kirsche", "Testbetrieb Müller", "maschinennummer", "anliegen_id"} {
		if strings.Contains(transcript, merkmal) {
			gefunden = append(gefunden, merkmal)
		}
	}
	notiz(ctx, "pruefung4_merkmale_im_transkript", gefunden)
	schreibeJSON(w, http.StatusOK, map[string]any{"empfangen": true})
}
