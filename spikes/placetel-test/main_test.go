package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testToken = "geheim-123"

func testServer(t *testing.T, versuche int) (*httptest.Server, *Protokoll) {
	t.Helper()
	p := NeuesProtokoll(1000, io.Discard)
	k := Konfig{Token: testToken, FotoVersuche: versuche, FotoLeerlauf: 10 * time.Minute, ProtokollMax: 1000}
	srv := httptest.NewServer(NeuerRouter(k, p))
	t.Cleanup(srv.Close)
	return srv, p
}

func anfrage(t *testing.T, srv *httptest.Server, method, pfad, body string, header map[string]string) (int, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(method, srv.URL+pfad, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range header {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	b, _ := io.ReadAll(res.Body)
	_ = json.Unmarshal(b, &out)
	return res.StatusCode, out
}

var bearer = map[string]string{"Authorization": "Bearer " + testToken}

func TestNormalisiereMaschinennummer(t *testing.T) {
	faelle := map[string]string{
		"TKKC9901":      "TKKC9901",
		"tkkc 9901":     "TKKC9901",
		"XXX36.1024":    "XXX361024",
		"xxx-36.10 24":  "XXX361024",
		"WK-2019-00457": "WK201900457",
		"12 34 56 78":   "12345678",
	}
	for ein, soll := range faelle {
		if ist := NormalisiereMaschinennummer(ein); ist != soll {
			t.Errorf("NormalisiereMaschinennummer(%q) = %q, soll %q", ein, ist, soll)
		}
	}
}

func TestMaschineSuchen(t *testing.T) {
	faelle := []struct {
		ein      string
		ergebnis string
		anzahl   int
	}{
		{"tkkc-9901", "gefunden", 1},
		{"36.1024", "gefunden", 1},
		{"x36 1024", "gefunden", 1},
		{"12345678", "mehrdeutig", 2},
		{"99999999", "nicht_gefunden", 0},
		{"01", "zu_kurz", 0},
	}
	for _, f := range faelle {
		out := MaschineSuchen(MaschineSuchenIn{Maschinennummer: f.ein})
		if out.Ergebnis != f.ergebnis || len(out.Maschine) != f.anzahl {
			t.Errorf("MaschineSuchen(%q) = %s/%d, soll %s/%d", f.ein, out.Ergebnis, len(out.Maschine), f.ergebnis, f.anzahl)
		}
	}
}

func TestFotoSchluessel(t *testing.T) {
	faelle := []struct {
		in   FotoStatusIn
		soll string
	}{
		{FotoStatusIn{Anrufer: "+49 170 123-4567"}, "tel:+491701234567"},
		{FotoStatusIn{Anrufer: "%%caller_number%%", Rufnummer: "0170 1234567"}, "tel:01701234567"},
		{FotoStatusIn{Anrufer: "%%caller_number%%", AnliegenID: "4711"}, "anliegen:4711"},
		{FotoStatusIn{}, "standard"},
	}
	for _, f := range faelle {
		if ist := FotoSchluessel(f.in); ist != f.soll {
			t.Errorf("FotoSchluessel(%+v) = %q, soll %q", f.in, ist, f.soll)
		}
	}
}

func TestFotoZaehlerUndLeerlauf(t *testing.T) {
	jetzt := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	f := NeuerFotoSpeicher(3, 10*time.Minute)
	f.jetzt = func() time.Time { return jetzt }
	in := FotoStatusIn{Anrufer: "+491701234567"}

	for i := 1; i <= 3; i++ {
		out, _, nr := f.Abfragen(in)
		if out.Status != "noch_nicht_da" || nr != i {
			t.Fatalf("Abfrage %d: %s (nr %d), soll noch_nicht_da", i, out.Status, nr)
		}
	}
	out, _, _ := f.Abfragen(in)
	if out.Status != "erkannt" || out.Maschinennummer != "TKKC9901" {
		t.Fatalf("Abfrage 4: %+v, soll erkannt/TKKC9901", out)
	}
	// Anderer Anrufer hat eigenen Zähler.
	if out, _, _ := f.Abfragen(FotoStatusIn{Anrufer: "+499999"}); out.Status != "noch_nicht_da" {
		t.Fatalf("anderer Anrufer: %s, soll noch_nicht_da", out.Status)
	}
	// Nach 10 Minuten Ruhe beginnt der Zähler neu.
	jetzt = jetzt.Add(11 * time.Minute)
	if out, _, nr := f.Abfragen(in); out.Status != "noch_nicht_da" || nr != 1 {
		t.Fatalf("nach Leerlauf: %s (nr %d), soll noch_nicht_da/1", out.Status, nr)
	}
	// Reset setzt zurück.
	f.Abfragen(in)
	f.Zuruecksetzen("")
	if _, _, nr := f.Abfragen(in); nr != 1 {
		t.Fatalf("nach Reset: nr %d, soll 1", nr)
	}
}

func TestAuth(t *testing.T) {
	srv, _ := testServer(t, 3)
	faelle := []struct {
		name   string
		header map[string]string
		soll   int
	}{
		{"ohne Token", nil, http.StatusUnauthorized},
		{"falscher Bearer", map[string]string{"Authorization": "Bearer falsch"}, http.StatusUnauthorized},
		{"Bearer", bearer, http.StatusOK},
		{"bearer klein", map[string]string{"Authorization": "bearer " + testToken}, http.StatusOK},
		{"X-API-Key", map[string]string{"X-API-Key": testToken}, http.StatusOK},
		{"falscher X-API-Key", map[string]string{"X-API-Key": "nein"}, http.StatusUnauthorized},
	}
	for _, f := range faelle {
		if code, _ := anfrage(t, srv, "POST", "/api/maschine-suchen", `{"maschinennummer":"TKKC9901"}`, f.header); code != f.soll {
			t.Errorf("%s: Status %d, soll %d", f.name, code, f.soll)
		}
	}
	res, err := http.Get(srv.URL + "/health")
	if err != nil || res.StatusCode != http.StatusOK {
		t.Fatalf("/health ohne Token: %v %v", res, err)
	}
}

func TestFotoStatusHTTPUndProtokoll(t *testing.T) {
	srv, _ := testServer(t, 2)
	body := `{"anrufer":"%%caller_number%%","rufnummer":"+49 170 1"}`
	for i := 0; i < 2; i++ {
		_, out := anfrage(t, srv, "POST", "/api/foto-status", body, bearer)
		if out["status"] != "noch_nicht_da" {
			t.Fatalf("Abfrage %d: %v", i+1, out)
		}
	}
	_, out := anfrage(t, srv, "POST", "/api/foto-status", body, bearer)
	if out["status"] != "erkannt" || out["maschinennummer"] != "TKKC9901" {
		t.Fatalf("Abfrage 3: %v", out)
	}

	code, prot := anfrage(t, srv, "GET", "/protokoll?pfad=/api/foto-status", "", map[string]string{"X-API-Key": testToken})
	if code != http.StatusOK || prot["anzahl"].(float64) != 3 {
		t.Fatalf("Protokoll: %d %v", code, prot)
	}
	erster := prot["eintraege"].([]any)[0].(map[string]any)
	notizen := erster["notizen"].(map[string]any)
	if !strings.HasPrefix(notizen["fester_parameter_anrufer"].(string), "NICHT ersetzt") {
		t.Errorf("Notiz fester Parameter: %v", notizen)
	}
	auth := erster["header"].(map[string]any)["Authorization"].([]any)[0].(string)
	if strings.Contains(auth, testToken) || !strings.Contains(auth, "korrekt") {
		t.Errorf("Token nicht korrekt maskiert: %q", auth)
	}

	// Filter seit= in der Zukunft liefert nichts; DELETE leert.
	_, prot = anfrage(t, srv, "GET", "/protokoll?seit=2999-01-01T00:00:00Z", "", bearer)
	if prot["anzahl"].(float64) != 0 {
		t.Errorf("seit-Filter: %v", prot["anzahl"])
	}
	anfrage(t, srv, "DELETE", "/protokoll", "", bearer)
	_, prot = anfrage(t, srv, "GET", "/protokoll", "", bearer)
	if prot["anzahl"].(float64) != 1 { // nur das DELETE selbst
		t.Errorf("nach DELETE: %v Einträge, soll 1", prot["anzahl"])
	}
}

func TestInboundUndAnliegenID(t *testing.T) {
	srv, p := testServer(t, 3)
	_, out := anfrage(t, srv, "POST", "/webhook/inbound", `{"caller_number":"+491701234567"}`, bearer)
	id, _ := out["anliegen_id"].(string)
	if len(id) != 4 {
		t.Fatalf("anliegen_id: %v", out)
	}
	anfrage(t, srv, "POST", "/api/maschine-suchen", `{"maschinennummer":"9901","anliegen_id":"`+id+`"}`, bearer)
	liste := p.Eintraege(time.Time{}, "/api/maschine-suchen")
	if len(liste) != 1 || !strings.Contains(liste[0].Notizen["anliegen_id"].(string), "bekannt aus Inbound Webhook") {
		t.Fatalf("Notiz anliegen_id: %+v", liste)
	}
}

func TestNachbearbeitung(t *testing.T) {
	srv, p := testServer(t, 3)
	body := `{"conversationId":"abc123","conversation_link":"https://x/conversations?conversation_id=abc123","transcript":"Agent: Die Maschinennummer ist TKKC9901."}`
	if code, _ := anfrage(t, srv, "POST", "/webhook/nachbearbeitung", body, bearer); code != http.StatusOK {
		t.Fatalf("Status %d", code)
	}
	// Auch kaputtes JSON (unmaskiertes Transkript) muss 200 liefern und protokolliert werden.
	if code, _ := anfrage(t, srv, "POST", "/webhook/nachbearbeitung", "{\"transcript\":\"er sagte \"hallo\"\"}", bearer); code != http.StatusOK {
		t.Fatalf("Status bei kaputtem JSON %d", code)
	}
	liste := p.Eintraege(time.Time{}, "/webhook/nachbearbeitung")
	if got := liste[0].Notizen["pruefung5_conversationId"]; !strings.Contains(got.(string), "GLEICH") {
		t.Errorf("Prüfung 5: %v", got)
	}
	if got := liste[1].Notizen["json_gueltig"].(string); !strings.HasPrefix(got, "NEIN") {
		t.Errorf("json_gueltig: %v", got)
	}
}

func TestVerzoegert(t *testing.T) {
	srv, _ := testServer(t, 3)
	_, out := anfrage(t, srv, "POST", "/api/verzoegert/0", "", bearer)
	if out["ergebnis"] != "fertig" {
		t.Fatalf("verzoegert/0: %v", out)
	}
	if code, _ := anfrage(t, srv, "POST", "/api/verzoegert/999", "", bearer); code != http.StatusBadRequest {
		t.Fatalf("verzoegert/999: %d", code)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := Verzoegert(ctx, 5); err == nil {
		t.Fatal("Abbruch nicht erkannt")
	}
}

// TestMCP spielt initialize, tools/list und tools/call als rohe JSON-RPC-POSTs durch.
func TestMCP(t *testing.T) {
	srv, p := testServer(t, 3)
	rpc := func(body string) (int, string) {
		req, _ := http.NewRequest("POST", srv.URL+"/mcp", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+testToken)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Mcp-Protocol-Version", "2025-06-18")
		req.Header.Set("X-Sitzung", "test-sitzung")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(b)
	}
	code, body := rpc(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"testclient","version":"1"},"_meta":{"sitzung":"xyz"}}}`)
	if code != http.StatusOK || !strings.Contains(body, "kumm-placetel-test") {
		t.Fatalf("initialize: %d %s", code, body)
	}
	code, body = rpc(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	for _, name := range []string{"foto_status", "maschine_suchen", "verzoegert"} {
		if !strings.Contains(body, name) {
			t.Fatalf("tools/list ohne %s: %d %s", name, code, body)
		}
	}
	code, body = rpc(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"maschine_suchen","arguments":{"maschinennummer":"tkkc 9901"},"_meta":{"sitzung":"xyz"}}}`)
	if code != http.StatusOK || !strings.Contains(body, "Testbetrieb Müller") {
		t.Fatalf("tools/call: %d %s", code, body)
	}

	var mcpEintraege []Eintrag
	for _, e := range p.Eintraege(time.Time{}, "/mcp") {
		if e.Art == "mcp" {
			mcpEintraege = append(mcpEintraege, e)
		}
	}
	if len(mcpEintraege) < 3 {
		t.Fatalf("MCP-Einträge: %d", len(mcpEintraege))
	}
	j, _ := json.Marshal(mcpEintraege)
	for _, soll := range []string{`"testclient"`, `"sitzung":"xyz"`, `"werkzeug":"maschine_suchen"`, `"X-Sitzung":["test-sitzung"]`} {
		if !bytes.Contains(j, []byte(soll)) {
			t.Errorf("MCP-Protokoll enthält nicht %s: %s", soll, j)
		}
	}
	// Ohne Token: 401.
	req, _ := http.NewRequest("POST", srv.URL+"/mcp", strings.NewReader(`{}`))
	res, _ := http.DefaultClient.Do(req)
	if res.StatusCode != http.StatusUnauthorized {
		t.Errorf("MCP ohne Token: %d", res.StatusCode)
	}
}
