package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadOnlyTransportBlocksWrites(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	defer srv.Close()
	c := NewClient(srv.URL, "x", 0)
	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		req, _ := http.NewRequest(m, srv.URL, strings.NewReader("{}"))
		if _, err := c.hc.Do(req); err == nil {
			t.Errorf("%s wurde nicht blockiert", m)
		}
	}
	if called {
		t.Error("Server hat eine schreibende Anfrage erhalten")
	}
}

// fakeV3 bildet die dokumentierten Antworten der klassischen API nach.
func fakeV3(t *testing.T) *httptest.Server {
	t.Helper()
	routes := map[string]string{
		"/teams":                         `[{"id":"T1","name":"Kumm"}]`,
		"/teams/T1/databases":            `[{"id":"db1","name":"Service"}]`,
		"/teams/T1/databases/db1/tables": `[{"id":"A","name":"Kunden","fields":[{"id":"A","name":"Name","type":"string"}]},{"id":"B","name":"Maschinen","fields":[{"id":"A","name":"Maschinennummer","type":"string"},{"id":"B","name":"Kunde","type":"ref"}]}]`,
		"/teams/T1/databases/db1": `{"settings":{"name":"Service"},"schema":{"types":{
			"A":{"caption":"Kunden","fields":{"A":{"base":"string","caption":"Name"},"C":{"base":"rev","caption":"Maschinen","refTypeId":"B","refFieldId":"B"}}},
			"B":{"caption":"Maschinen","fields":{"A":{"base":"string","caption":"Maschinennummer"},"B":{"base":"ref","caption":"Kunde","refTypeId":"A"}}}}}}`,
		"/teams/T1/databases/db1/tables/B/records": `[{"id":1,"fields":{"Maschinennummer":"TKKC9901","Kunde":1}},{"id":2,"fields":{"Maschinennummer":12345678}},{"id":3,"fields":{}}]`,
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unerwartete Methode %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer geheim" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		body, ok := routes[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
}

func TestV3DiscoverAndValues(t *testing.T) {
	srv := fakeV3(t)
	defer srv.Close()
	src := &sourceV3{c: NewClient(srv.URL, "geheim", 0)}
	cs, err := src.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 1 || len(cs[0].Tables) != 2 {
		t.Fatalf("unerwartete Struktur: %+v", cs)
	}
	_, tbl, f, err := findField(cs, "maschinen", "Kunde")
	if err != nil {
		t.Fatal(err)
	}
	if f.RefTable != "Kunden (A)" {
		t.Errorf("RefTable = %q, want %q", f.RefTable, "Kunden (A)")
	}
	// Rück-Verknüpfung, die nur im Schema steht, wird ergänzt.
	var rev *Field
	for i, f := range cs[0].Tables[0].Fields {
		if f.ID == "C" {
			rev = &cs[0].Tables[0].Fields[i]
		}
	}
	if rev == nil || rev.Type != "rev" || rev.RefTable != "Maschinen (B)" {
		t.Errorf("Rück-Verknüpfung fehlt oder falsch: %+v", rev)
	}

	_, _, nr, _ := findField(cs, "B", "Maschinennummer")
	vals, err := src.Values(context.Background(), tbl, nr, 0)
	if err != nil {
		t.Fatal(err)
	}
	a := Analyse(vals, 10)
	if a.Records != 3 || a.Evaluated != 2 || a.Kinds["Zahl"] != 1 {
		t.Errorf("Analyse = %+v", a)
	}

	report := SchemaReport(ReportMeta{Stats: &src.c.Stats}, cs)
	for _, want := range []string{"Maschinennummer", "Kunden (A)", "## Verknüpfungen"} {
		if !strings.Contains(report, want) {
			t.Errorf("Schema-Bericht enthält %q nicht", want)
		}
	}
	if strings.Contains(report, "geheim") {
		t.Error("Schema-Bericht enthält den Schlüssel")
	}
}

func TestV4DiscoverAndValues(t *testing.T) {
	routes := map[string]string{
		"/workspace/ws0000000001/modules":                                  `{"data":[{"name":"service"}],"page_info":{"has_more":false}}`,
		"/workspace/ws0000000001/modules/service/tables":                   `{"data":[{"name":"maschinen"}],"page_info":{"has_more":false}}`,
		"/workspace/ws0000000001/modules/service/tables/maschinen/fields":  `{"data":[{"name":"nummer","type":"string"},{"name":"kunde","type":"reference","refTableName":"kunden"}],"page_info":{"has_more":false}}`,
		"/workspace/ws0000000001/modules/service/tables/maschinen/records": `{"data":[{"id":"1","values":{"nummer":"TKKC9901"}},{"id":"2","values":{"nummer":"XXX36.1024"}}],"page_info":{"has_more":false}}`,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("limit") == "" {
			t.Errorf("limit fehlt: %s", r.URL)
		}
		body, ok := routes[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(body))
	}))
	defer srv.Close()
	src := &sourceV4{c: NewClient(srv.URL, "k", 0), workspaceID: "ws0000000001"}
	cs, err := src.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	_, tbl, f, err := findField(cs, "maschinen", "kunde")
	if err != nil {
		t.Fatal(err)
	}
	if f.RefTable != "kunden" {
		t.Errorf("RefTable = %q", f.RefTable)
	}
	_, _, nr, _ := findField(cs, "maschinen", "nummer")
	vals, err := src.Values(context.Background(), tbl, nr, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(vals) != 1 {
		t.Errorf("max=1 lieferte %d Werte", len(vals))
	}
}
