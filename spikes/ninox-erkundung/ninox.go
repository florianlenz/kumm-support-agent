package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// Einheitliches Modell für beide Ninox-APIs:
//   - Ninox 3 („klassisch“): Team → Datenbank → Tabelle → Feld,
//     https://api.ninox.com/v1/teams/{team}/databases/{db}/tables …
//   - Ninox 4: Workspace → Modul → Tabelle → Feld,
//     https://go.ninox.com/api/v1/workspace/{ws}/modules/{modul}/tables …

// Container ist eine Datenbank (Ninox 3) oder ein Modul (Ninox 4).
type Container struct {
	Label  string // z. B. "Team „Kumm“ › Datenbank „Service“"
	Tables []Table
	Notes  []string // Hinweise, z. B. wenn das Schema nicht geladen werden konnte
	Raw    any      // Rohes Schema (ohne Datensätze) für die lokale Ablage
	RawTag string   // Dateiname-Teil für die Rohablage
}

// Table ist eine Tabelle mit ihren Feldern.
type Table struct {
	ID     string // Ninox 3: "A", "B" …; Ninox 4: Tabellenname
	Name   string
	Fields []Field

	// Ort der Tabelle für Datensatz-Abfragen.
	teamID, dbID, module string
}

// Field ist die Definition eines Feldes.
type Field struct {
	ID       string
	Name     string
	Type     string
	RefTable string         // Zieltabelle einer Verknüpfung (falls erkennbar)
	Details  map[string]any // weitere Angaben aus dem Schema
}

// Source ist eine der beiden Ninox-APIs.
type Source interface {
	Name() string
	Discover(ctx context.Context) ([]Container, error)
	// Values liest bis zu max Datensätze (0 = alle) und liefert den Wert von
	// field je Datensatz (nil, wenn leer).
	Values(ctx context.Context, t Table, field Field, max int) ([]any, error)
}

// ---------------------------------------------------------------- Ninox 3 --

type sourceV3 struct {
	c            *Client
	teamID, dbID string
}

func (s *sourceV3) Name() string { return "Ninox 3 (klassische API, /v1/teams/…)" }

type idName struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (s *sourceV3) Discover(ctx context.Context) ([]Container, error) {
	var teams []idName
	if err := s.c.Get(ctx, "/teams", nil, &teams); err != nil {
		return nil, fmt.Errorf("Teams lesen: %w", err)
	}
	if s.teamID != "" {
		teams = filterIDName(teams, s.teamID)
		if len(teams) == 0 {
			return nil, fmt.Errorf("Team %q nicht gefunden (NINOX_TEAM_ID prüfen)", s.teamID)
		}
	}
	var out []Container
	for _, team := range teams {
		var dbs []idName
		if err := s.c.Get(ctx, "/teams/"+url.PathEscape(team.ID)+"/databases", nil, &dbs); err != nil {
			return nil, fmt.Errorf("Datenbanken von Team %s lesen: %w", team.ID, err)
		}
		if s.dbID != "" {
			dbs = filterIDName(dbs, s.dbID)
		}
		for _, db := range dbs {
			c, err := s.database(ctx, team, db)
			if err != nil {
				return nil, err
			}
			out = append(out, c)
		}
	}
	if s.dbID != "" && len(out) == 0 {
		return nil, fmt.Errorf("Datenbank %q nicht gefunden (NINOX_DB_ID prüfen)", s.dbID)
	}
	return out, nil
}

func (s *sourceV3) database(ctx context.Context, team, db idName) (Container, error) {
	c := Container{
		Label:  fmt.Sprintf("Team „%s“ (%s) › Datenbank „%s“ (%s)", team.Name, team.ID, db.Name, db.ID),
		RawTag: "v3-" + db.ID,
	}
	base := "/teams/" + url.PathEscape(team.ID) + "/databases/" + url.PathEscape(db.ID)

	var tables []struct {
		ID     string           `json:"id"`
		Name   string           `json:"name"`
		Fields []map[string]any `json:"fields"`
	}
	if err := s.c.Get(ctx, base+"/tables", nil, &tables); err != nil {
		return c, fmt.Errorf("Tabellen von %s lesen: %w", db.ID, err)
	}

	// Das Datenbankschema enthält zusätzliche Angaben, u. a. die Zieltabelle
	// von Verknüpfungen (laut Community-Beispielen "refTypeId"; unbestätigt).
	var schema map[string]any
	schemaFields := map[string]map[string]map[string]any{}
	if err := s.c.Get(ctx, base, nil, &schema); err != nil {
		c.Notes = append(c.Notes, "Datenbankschema nicht lesbar ("+err.Error()+"); Verknüpfungsziele fehlen evtl.")
	} else {
		c.Raw = schema
		schemaFields = v3SchemaFields(schema)
	}

	names := map[string]string{}
	for _, t := range tables {
		names[t.ID] = t.Name
	}
	for _, t := range tables {
		tbl := Table{ID: t.ID, Name: t.Name, teamID: team.ID, dbID: db.ID}
		seen := map[string]bool{}
		for _, raw := range t.Fields {
			f := fieldFromMap(raw, schemaFields[t.ID][str(raw["id"])])
			seen[f.ID] = true
			tbl.Fields = append(tbl.Fields, f)
		}
		// Felder, die nur im Schema stehen (z. B. Rück-Verknüpfungen, falls
		// /tables sie nicht liefert).
		for _, id := range sortedKeys(schemaFields[t.ID]) {
			if seen[id] {
				continue
			}
			sf := schemaFields[t.ID][id]
			f := fieldFromMap(map[string]any{"id": id, "name": sf["caption"], "type": sf["base"]}, sf)
			f.Details["nurImSchema"] = true
			tbl.Fields = append(tbl.Fields, f)
		}
		for i := range tbl.Fields {
			if n, ok := names[tbl.Fields[i].RefTable]; ok {
				tbl.Fields[i].RefTable = fmt.Sprintf("%s (%s)", n, tbl.Fields[i].RefTable)
			}
		}
		c.Tables = append(c.Tables, tbl)
	}
	return c, nil
}

// v3SchemaFields liest schema.types[tabelle].fields[feld] aus der Antwort
// von GET /teams/{t}/databases/{db}. Die Antwort hat laut Doku entweder die
// Form {settings, schema:{types…}} oder direkt {types…}.
func v3SchemaFields(root map[string]any) map[string]map[string]map[string]any {
	out := map[string]map[string]map[string]any{}
	sch := root
	if inner, ok := root["schema"].(map[string]any); ok {
		sch = inner
	}
	types, _ := sch["types"].(map[string]any)
	for tid, tv := range types {
		tm, _ := tv.(map[string]any)
		fields, _ := tm["fields"].(map[string]any)
		out[tid] = map[string]map[string]any{}
		for fid, fv := range fields {
			if fm, ok := fv.(map[string]any); ok {
				out[tid][fid] = fm
			}
		}
	}
	return out
}

func (s *sourceV3) Values(ctx context.Context, t Table, field Field, max int) ([]any, error) {
	const perPage = 500
	path := fmt.Sprintf("/teams/%s/databases/%s/tables/%s/records",
		url.PathEscape(t.teamID), url.PathEscape(t.dbID), url.PathEscape(t.ID))
	var out []any
	for page := 0; ; page++ {
		q := url.Values{"perPage": {fmt.Sprint(perPage)}, "page": {fmt.Sprint(page)}}
		var recs []struct {
			ID     json.Number    `json:"id"`
			Fields map[string]any `json:"fields"`
		}
		if err := s.c.Get(ctx, path, q, &recs); err != nil {
			return out, err
		}
		for _, r := range recs {
			v, ok := r.Fields[field.Name]
			if !ok {
				v = r.Fields[field.ID]
			}
			out = append(out, v)
			if max > 0 && len(out) >= max {
				return out, nil
			}
		}
		if len(recs) < perPage {
			return out, nil
		}
	}
}

// ---------------------------------------------------------------- Ninox 4 --

type sourceV4 struct {
	c           *Client
	workspaceID string
	module      string
}

func (s *sourceV4) Name() string { return "Ninox 4 (Public API, /api/v1/workspace/…)" }

type pageInfo struct {
	HasMore bool `json:"has_more"`
}

func (s *sourceV4) wsPath() string { return "/workspace/" + url.PathEscape(s.workspaceID) }

// pages ruft eine Liste mit limit/offset ab, bis has_more false ist.
func (s *sourceV4) pages(ctx context.Context, path string, q url.Values, each func(json.RawMessage) error) error {
	const limit = 100
	for offset := 0; ; offset += limit {
		qq := url.Values{}
		for k, v := range q {
			qq[k] = v
		}
		qq.Set("limit", fmt.Sprint(limit))
		qq.Set("offset", fmt.Sprint(offset))
		var resp struct {
			Data     []json.RawMessage `json:"data"`
			PageInfo pageInfo          `json:"page_info"`
		}
		if err := s.c.Get(ctx, path, qq, &resp); err != nil {
			return err
		}
		for _, d := range resp.Data {
			if err := each(d); err != nil {
				return err
			}
		}
		if !resp.PageInfo.HasMore || len(resp.Data) == 0 {
			return nil
		}
	}
}

func (s *sourceV4) Discover(ctx context.Context) ([]Container, error) {
	var modules []string
	if s.module != "" {
		modules = []string{s.module}
	} else {
		err := s.pages(ctx, s.wsPath()+"/modules", nil, func(m json.RawMessage) error {
			var mod struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(m, &mod); err != nil {
				return err
			}
			modules = append(modules, mod.Name)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("Module lesen: %w", err)
		}
	}

	var out []Container
	for _, mod := range modules {
		c := Container{Label: fmt.Sprintf("Workspace %s › Modul „%s“", s.workspaceID, mod), RawTag: "v4-" + mod}
		modPath := s.wsPath() + "/modules/" + url.PathEscape(mod)
		var tableNames []string
		err := s.pages(ctx, modPath+"/tables", nil, func(m json.RawMessage) error {
			var t struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(m, &t); err != nil {
				return err
			}
			tableNames = append(tableNames, t.Name)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("Tabellen von Modul %s lesen: %w", mod, err)
		}
		raw := map[string]any{}
		for _, tn := range tableNames {
			tbl := Table{ID: tn, Name: tn, module: mod}
			var rawFields []map[string]any
			err := s.pages(ctx, modPath+"/tables/"+url.PathEscape(tn)+"/fields", nil, func(m json.RawMessage) error {
				var f map[string]any
				dec := json.NewDecoder(strings.NewReader(string(m)))
				dec.UseNumber()
				if err := dec.Decode(&f); err != nil {
					return err
				}
				rawFields = append(rawFields, f)
				tbl.Fields = append(tbl.Fields, fieldFromMap(f, nil))
				return nil
			})
			if err != nil {
				return nil, fmt.Errorf("Felder von %s.%s lesen: %w", mod, tn, err)
			}
			raw[tn] = rawFields
			c.Tables = append(c.Tables, tbl)
		}
		c.Raw = raw
		out = append(out, c)
	}
	return out, nil
}

func (s *sourceV4) Values(ctx context.Context, t Table, field Field, max int) ([]any, error) {
	path := s.wsPath() + "/modules/" + url.PathEscape(t.module) + "/tables/" + url.PathEscape(t.ID) + "/records"
	var out []any
	errStop := fmt.Errorf("genug")
	err := s.pages(ctx, path, url.Values{"fields": {field.Name}}, func(m json.RawMessage) error {
		var row struct {
			Values map[string]any `json:"values"`
		}
		dec := json.NewDecoder(strings.NewReader(string(m)))
		dec.UseNumber()
		if err := dec.Decode(&row); err != nil {
			return err
		}
		out = append(out, row.Values[field.Name])
		if max > 0 && len(out) >= max {
			return errStop
		}
		return nil
	})
	if err == errStop {
		err = nil
	}
	return out, err
}

// ---------------------------------------------------------------- Helfer --

// fieldFromMap baut ein Field aus der API-Antwort (primary) und optional
// ergänzenden Schema-Angaben (extra, nur Ninox 3).
func fieldFromMap(primary, extra map[string]any) Field {
	f := Field{
		ID:      str(primary["id"]),
		Name:    str(primary["name"]),
		Type:    str(primary["type"]),
		Details: map[string]any{},
	}
	if f.ID == "" {
		f.ID = f.Name // Ninox 4 kennt keine Feld-IDs, nur Namen
	}
	merge := func(m map[string]any) {
		for k, v := range m {
			switch k {
			case "id", "name", "type", "caption", "captions", "labels":
				continue
			}
			if _, exists := f.Details[k]; !exists {
				f.Details[k] = v
			}
		}
	}
	merge(primary)
	merge(extra)
	if f.Name == "" && extra != nil {
		f.Name = str(extra["caption"])
	}
	if f.Type == "" && extra != nil {
		f.Type = str(extra["base"])
	}
	// Verknüpfungsziel: Ninox 4 "refTableName"; Ninox 3 vermutlich "refTypeId".
	for _, k := range []string{"refTableName", "refTypeId", "refTableId", "refTable"} {
		if v := str(f.Details[k]); v != "" {
			f.RefTable = v
			break
		}
	}
	return f
}

func str(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	default:
		return fmt.Sprint(t)
	}
}

func filterIDName(list []idName, id string) []idName {
	var out []idName
	for _, x := range list {
		if x.ID == id {
			out = append(out, x)
		}
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
