package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// NeuerMCPHandler baut den MCP-Server (Streamable HTTP, stateless, JSON-Antworten)
// mit denselben drei Werkzeugen wie die API-Endpunkte.
func NeuerMCPHandler(d *Dienst, p *Protokoll, token string) http.Handler {
	server := mcp.NewServer(&mcp.Implementation{Name: "kumm-placetel-test", Version: "0.1.0"}, nil)
	server.AddReceivingMiddleware(mcpProtokoll(p, token))

	mcp.AddTool(server, &mcp.Tool{
		Name: "foto_status",
		Description: "Fragt ab, ob das Foto vom Typenschild schon per WhatsApp angekommen ist. " +
			"Liefert status=noch_nicht_da (dann nach einigen Sekunden erneut fragen) oder status=erkannt mit Maschinennummer und Kunde.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in FotoStatusIn) (*mcp.CallToolResult, FotoStatusOut, error) {
		anliegenNotiz(ctx, d.Anliegen, in.AnliegenID)
		out, schluessel, nr := d.Foto.Abfragen(in)
		notiz(ctx, "foto_schluessel", schluessel)
		notiz(ctx, "foto_abfrage_nr", nr)
		return nil, out, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "maschine_suchen",
		Description: "Sucht Maschine und Kunde anhand der letzten 8 Zeichen der Maschinennummer vom Typenschild.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in MaschineSuchenIn) (*mcp.CallToolResult, MaschineSuchenOut, error) {
		anliegenNotiz(ctx, d.Anliegen, in.AnliegenID)
		return nil, MaschineSuchen(in), nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "verzoegert",
		Description: "Testwerkzeug: antwortet erst nach der angegebenen Anzahl Sekunden mit einem Kennwort.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in VerzoegertIn) (*mcp.CallToolResult, VerzoegertOut, error) {
		anliegenNotiz(ctx, d.Anliegen, in.AnliegenID)
		out, err := Verzoegert(ctx, in.Sekunden)
		return nil, out, err
	})

	return mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true, Logger: slog.Default()},
	)
}

// mcpProtokoll schreibt pro MCP-Nachricht einen eigenen Eintrag (Art "mcp") mit
// Methode, _meta, clientInfo, Protokollversion, Werkzeug, Argumenten, Headern
// und Ergebnis.
func mcpProtokoll(p *Protokoll, token string) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			start := time.Now()
			details := map[string]any{}
			if params := req.GetParams(); params != nil {
				if meta := params.GetMeta(); len(meta) > 0 {
					details["_meta"] = meta
				}
			}
			switch params := req.GetParams().(type) {
			case *mcp.InitializeParams:
				details["clientInfo"] = params.ClientInfo
				details["protocolVersion"] = params.ProtocolVersion
				details["capabilities"] = params.Capabilities
			case *mcp.CallToolParamsRaw:
				details["werkzeug"] = params.Name
				details["argumente"] = json.RawMessage(params.Arguments)
			}
			if s := req.GetSession(); s != nil && s.ID() != "" {
				details["session"] = s.ID()
			}
			var header map[string][]string
			if extra := req.GetExtra(); extra != nil && extra.Header != nil {
				header = maskiert(extra.Header, token)
			}

			res, err := next(ctx, method, req)

			var antwort any = res
			if err != nil {
				antwort = map[string]any{"fehler": err.Error()}
			}
			if n, ok := ctx.Value(notizenKey{}).(*notizen); ok {
				for k, v := range n.kopie() {
					details["notiz_"+k] = v
				}
			}
			p.Hinzufuegen(Eintrag{
				Zeit:    start,
				Art:     "mcp",
				Methode: method,
				Pfad:    "/mcp",
				Header:  header,
				Body:    details,
				Antwort: antwort,
				DauerMs: time.Since(start).Milliseconds(),
			})
			return res, err
		}
	}
}
