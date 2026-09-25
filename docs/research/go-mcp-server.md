# Go: MCP-Server mit Streamable HTTP und Header-Auswertung

Recherche zu [#4](https://github.com/florianlenz/kumm-support-agent/issues/4), Stand 2026-09-25.
Nur Primärquellen: Placetel-Doku, Quellcode/README der SDKs (lokal per `go mod download` geprüft), MCP-Spezifikation, Scalingo-Doku.

## Kurzfassung

- **SDK:** Das offizielle `github.com/modelcontextprotocol/go-sdk` nehmen. Aktuell **v1.8.0** (2026-09-14), stabil seit v1.0.0, unterstützt Spezifikation 2026-07-28 und alle älteren Versionen zurück bis 2024-11-05. Braucht Go ≥ 1.25.0.
- **Absicherung:** Placetel sendet entweder `Authorization: Bearer <Token>` oder einen eigenen Header (z. B. `X-API-Key: <Wert>`). Eine einfache `net/http`-Middleware vor dem MCP-Handler reicht dafür.
- **Header protokollieren:** geht an zwei Stellen: als HTTP-Middleware (alle Header jeder Anfrage) und im Tool selbst über `req.Extra.Header`. MCP-Metadaten (`_meta`, `clientInfo`, Protokollversion, Tool-Argumente) lassen sich mit einer MCP-Middleware (`server.AddReceivingMiddleware`) protokollieren. Das Gerüst unten ist gebaut und lokal getestet.
- **„Sitzungskontext“:** kommt in der Placetel-Doku **nicht vor**. Dort ist nur beschrieben: Server-URL, Authentifizierung (keine / Bearer / eigener Header) und Tool-Erkennung per `tools/list`. Das Ausführen der Tools im Anruf ist laut Doku noch „in Vorbereitung“. Was Placetel tatsächlich mitschickt (Header, `_meta`), lässt sich nur mit dem Test-Dienst herausfinden, und genau dafür ist das Protokollieren gedacht.
- **Scalingo:** läuft. Go-Buildpack erkennt `go.mod`, Standardversion ist die neueste go1.25, `$PORT` wird gesetzt. Der Router lässt SSE/lange Verbindungen zu, bricht aber ab, wenn in den ersten **30 s** keine Antwort kommt oder später **59 s** lang keine Daten fließen. Empfehlung: `Stateless: true` + `JSONResponse: true`, dann ist jeder Tool-Aufruf ein normaler kurzer POST.

## 1. Was Placetel beim MCP-Server sendet

Quelle: <https://aipro.placetel.de/docs/de/mcp-server.md>

- Transport: „über HTTP bzw. Streamable-Transport“.
- Einrichtung: Name, **vollständige Endpunkt-URL inkl. Pfad** (z. B. `https://ihr-server.de/v1/mcp/core`; die reine Basis-URL führt zu „keine Tools gefunden“), Authentifizierung.
- Authentifizierung, wörtlich:
  - „**Bearer-Token:** Es wird der Header `Authorization: Bearer <Token>` gesendet.“
  - „**Eigener Header:** Der eingegebene Wert wird unter dem von Ihnen angegebenen Header-Namen gesendet (z. B. `X-API-Key: <Wert>`).“
  - „**Keine:** Es werden keine Zugangsdaten mitgesendet.“
- „Tools entdecken“ ruft `tools/list` auf. Die Tools werden global angelegt und pro Telefonassistent im Tools-Tab aktiviert. Tools mit einem Namen, den es im Account schon gibt, werden übersprungen.
- **Beta:** „Das **Ausführen dieser Tools während eines laufenden Anrufs** ist derzeit noch in Vorbereitung und **in Kürze verfügbar**.“
- Einen **„Sitzungskontext“** erwähnt die Seite nicht. Auch die übrigen Seiten im Doku-Index (<https://aipro.placetel.de/docs/llms.txt>: Tools, Prompting Guide, API-Anfragen, Inbound Webhook) erwähnen ihn nicht.

Verwandt, aber **nicht** für MCP dokumentiert: Beim **Inbound Webhook** gibt es Systemvariablen `{{caller_number}}`, `{{called_number}}` und `{{forwarded_from_number}}`, die sich auch in „zusätzlichen Headern“ einsetzen lassen (<https://aipro.placetel.de/docs/de/inbound-webhook.md>). Ob der Sitzungskontext der MCP-Oberfläche etwas Ähnliches ist (z. B. Werte als Header oder in `_meta`), ist offen. Das klärt der Test-Dienst.

**Folgerung:** Der Test-Dienst sollte *alles* protokollieren: alle HTTP-Header, `_meta` jeder MCP-Anfrage, `clientInfo` und `protocolVersion` aus `initialize` sowie die Tool-Argumente. Dann sieht man nach dem ersten echten Aufruf, was Placetel mitschickt.

## 2. SDK-Wahl

| | offizielles `modelcontextprotocol/go-sdk` | `mark3labs/mcp-go` |
|---|---|---|
| Aktuelle Version | v1.8.0 (2026-09-14) | v1.1.1 (2026-09-23), v1.0.0 erst am 2026-09-02 |
| Spezifikation | 2026-07-28 + 2025-11-25, 2025-06-18, 2025-03-26, 2024-11-05 | 2025-11-25 + drei ältere |
| Go-Mindestversion (`go.mod`) | 1.25.0 | 1.25.5 |
| Streamable HTTP | `mcp.NewStreamableHTTPHandler` (ein `http.Handler`) | `server.NewStreamableHTTPServer` |
| HTTP-Header im Tool | `req.Extra.Header` | `CallToolRequest.Header` |
| Selbstbeschreibung | offizielles SDK der MCP-Organisation | README: „under active development“ |

Quellen: <https://github.com/modelcontextprotocol/go-sdk> (README, Tabelle „Version Compatibility“), Releases per `gh api repos/modelcontextprotocol/go-sdk/releases`, <https://github.com/mark3labs/mcp-go> (README), Releases per `gh api repos/mark3labs/mcp-go/releases`. Signaturen aus dem Quellcode:

- go-sdk v1.8.0 `mcp/streamable.go`: `func NewStreamableHTTPHandler(getServer func(*http.Request) *Server, opts *StreamableHTTPOptions) *StreamableHTTPHandler`. Optionen u. a. `Stateless`, `JSONResponse`, `Logger *slog.Logger`, `SessionTimeout`, `MaxRequestBodyBytes` (Standard 4 MiB).
- go-sdk `mcp/shared.go`: `type RequestExtra struct { TokenInfo *auth.TokenInfo; Header http.Header; … }`. Der Streamable-Handler setzt `Header: req.Header` für jede Anfrage (`mcp/streamable.go`, „Include metadata for all requests“).
- go-sdk `mcp/shared.go`: `type Middleware func(MethodHandler) MethodHandler`, registriert über `(*Server).AddReceivingMiddleware`. Beispiel dafür in `examples/http/logging_middleware.go`.
- go-sdk `auth/auth.go`: `RequireBearerToken(verifier TokenVerifier, opts *RequireBearerTokenOptions)`. Diese Funktion ist auf OAuth zugeschnitten und lehnt ohne `AllowMissingExpiration` Tokens ohne Ablaufzeit ab. Für ein statisches Token ist eine eigene Middleware einfacher und deckt auch den „eigenen Header“ ab.
- mcp-go v1.1.1 `server/streamable_http.go`: `NewStreamableHTTPServer(server *MCPServer, opts ...StreamableHTTPOption)`, `WithHTTPContextFunc`, `WithStateLess`. `mcp/tools.go`: `CallToolRequest{ Header http.Header … }`.

**Empfehlung:** `modelcontextprotocol/go-sdk`. Es ist das offizielle SDK, reifer (v1.x seit über einem Jahr, aktuelle Spezifikation), der Handler ist ein normaler `http.Handler`, und Header und `_meta` sind ohne Umwege erreichbar.

### Hinweis Spezifikation 2026-07-28

Die neue Spezifikation macht Streamable HTTP sitzungslos: „each message is an HTTP POST to a single MCP endpoint; replies arrive as a JSON object or a request-scoped SSE stream“. Protokoll-Metadaten stehen im Body unter `_meta.io.modelcontextprotocol/*` und werden zusätzlich in HTTP-Header gespiegelt (<https://modelcontextprotocol.io/specification/2026-07-28/basic/transports>). Ältere Clients mit `initialize` bedient das SDK weiterhin. Welche Version Placetel spricht, zeigt das Protokoll (`protocolVersion` bzw. Header `Mcp-Protocol-Version`).

## 3. Code-Gerüst (getestet)

Mit Go 1.25.0 und go-sdk v1.8.0 gebaut, `go vet` sauber. Lokal mit `curl` getestet: ohne Token 401; `initialize` und `tools/call` antworten als JSON; `/api/maschine-suchen` liefert dasselbe Ergebnis; Header, `_meta`, `clientInfo` und Tool-Argumente erscheinen im Log.

Umfang: ein Tool (`maschine_suchen`), Auth über Bearer **oder** eigenen Header (per Umgebungsvariable), Header-Log, MCP-Metadaten-Log und dieselbe Fachlogik als einfacher HTTP-JSON-Endpunkt.

```go
package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- Fachlogik: einmal geschrieben, von MCP und HTTP genutzt ---

type MaschineSuchenIn struct {
	Maschinennummer string `json:"maschinennummer" jsonschema:"die letzten 8 Zeichen der Maschinennummer"`
}

type MaschineSuchenOut struct {
	Gefunden bool   `json:"gefunden"`
	Kunde    string `json:"kunde,omitempty"`
}

func maschineSuchen(ctx context.Context, in MaschineSuchenIn) (MaschineSuchenOut, error) {
	return MaschineSuchenOut{Gefunden: false}, nil // Platzhalter (Zwischenspeicher/Ninox)
}

// --- Geheimnisse vor dem Protokollieren maskieren ---

func masked(h http.Header) http.Header {
	h = h.Clone()
	for _, k := range []string{"Authorization", "X-Api-Key", os.Getenv("MCP_AUTH_HEADER")} {
		if k != "" && h.Get(k) != "" {
			h.Set(k, "***")
		}
	}
	return h
}

// --- HTTP-Middleware: alle Header protokollieren ---

func logHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Info("http", "method", r.Method, "path", r.URL.Path, "headers", masked(r.Header))
		next.ServeHTTP(w, r)
	})
}

// --- HTTP-Middleware: statisches Token (Bearer oder eigener Header) ---

func requireToken(headerName, token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get(headerName)
		if strings.EqualFold(headerName, "Authorization") {
			got = strings.TrimPrefix(got, "Bearer ")
		}
		if token == "" || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// --- MCP-Middleware: Methode, Session, _meta, clientInfo, Tool-Argumente ---

func logMCP(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		attrs := []any{"method", method, "session", req.GetSession().ID()}
		if p := req.GetParams(); p != nil {
			attrs = append(attrs, "meta", p.GetMeta())
		}
		switch p := req.GetParams().(type) {
		case *mcp.InitializeParams:
			if p.ClientInfo != nil {
				attrs = append(attrs, "client", p.ClientInfo.Name, "clientVersion", p.ClientInfo.Version)
			}
			attrs = append(attrs, "protocol", p.ProtocolVersion)
		case *mcp.CallToolParamsRaw:
			attrs = append(attrs, "tool", p.Name, "args", string(p.Arguments))
		}
		slog.Info("mcp", attrs...)
		return next(ctx, method, req)
	}
}

func main() {
	token := os.Getenv("MCP_TOKEN")
	authHeader := os.Getenv("MCP_AUTH_HEADER") // "Authorization" (Bearer) oder z. B. "X-Api-Key"
	if authHeader == "" {
		authHeader = "Authorization"
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "kumm-support", Version: "0.1.0"}, nil)
	server.AddReceivingMiddleware(logMCP)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "maschine_suchen",
		Description: "Sucht Maschine und Kunde anhand der letzten 8 Zeichen der Maschinennummer.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in MaschineSuchenIn) (*mcp.CallToolResult, MaschineSuchenOut, error) {
		slog.Info("tool", "headers", masked(req.Extra.Header)) // HTTP-Header auch im Tool verfügbar
		out, err := maschineSuchen(ctx, in)
		return nil, out, err // Ergebnis als structuredContent + Text
	})

	mcpHandler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true, Logger: slog.Default()},
	)

	mux := http.NewServeMux()
	mux.Handle("/mcp", requireToken(authHeader, token, mcpHandler))
	mux.Handle("POST /api/maschine-suchen", requireToken(authHeader, token, http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			var in MaschineSuchenIn
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			out, err := maschineSuchen(r.Context(), in)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(out)
		})))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })

	port := os.Getenv("PORT") // von Scalingo gesetzt
	if port == "" {
		port = "8080"
	}
	srv := &http.Server{Addr: ":" + port, Handler: logHeaders(mux), ReadHeaderTimeout: 10 * time.Second}
	slog.Info("listening", "port", port)
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("server", "err", err)
		os.Exit(1)
	}
}
```

Beispiel-Log aus dem lokalen Test (gekürzt; `X-Sitzung` und `_meta.sitzung` waren Testwerte):

```
INFO http method=POST path=/mcp headers="map[... Authorization:[***] ... X-Sitzung:[abc]]"
INFO mcp method=initialize session="" meta=map[sitzung:xyz] client=test clientVersion=1 protocol=2025-06-18
INFO mcp method=tools/call session="" meta=map[sitzung:xyz] tool=maschine_suchen args="{\"maschinennummer\":\"TKKC9901\"}"
```

Hinweise:

- `Stateless: true`: Der Server liest und setzt kein `Mcp-Session-Id`. GET/DELETE liefern 405. Anfragen vom Server an den Client sind nicht möglich; die brauchen wir auch nicht (Kommentar zu `StreamableHTTPOptions.Stateless`). Deshalb ist `session=""` im Log leer. Eine Zuordnung Werkzeug-Aufruf ↔ Anliegen muss dann über das kommen, was Placetel mitschickt (Header/`_meta`), und nicht über die MCP-Session.
- Braucht man doch eine MCP-Session (um mehrere Aufrufe eines Anrufs zu verknüpfen), `Stateless` weglassen. Die Session-ID steht dann in `req.GetSession().ID()`. Die Sessions liegen aber im Arbeitsspeicher einer Instanz: Das passt nur zu **einem** Container auf Scalingo oder braucht Sticky Sessions.
- `JSONResponse: true`: Antwort als `application/json` statt SSE. Bei kurzen Tool-Aufrufen ist das einfacher, und am Router gibt es nichts zu bedenken.

## 4. Scalingo

- **Buildpack:** Erkennt Go an `go.mod`. Verfügbar sind Go 1.25, 1.26 und 1.27, Standard ist „the latest go1.25 version“ (<https://doc.scalingo.com/languages/go/start>). Die Version lässt sich per `// +scalingo goVersion go1.26` in `go.mod` oder per Umgebungsvariable `GOVERSION` festlegen. Gebaut werden automatisch alle `main`-Pakete, alternativ gezielt per `// +scalingo install ./cmd/...` (<https://github.com/Scalingo/go-buildpack/blob/master/README.md>). go-sdk v1.8.0 braucht Go ≥ 1.25.0, das passt also schon zum Standard.
- **Procfile:** `web: <base package name>`, z. B. `web: example` für `github.com/user/example` (<https://doc.scalingo.com/languages/go/start>).
- **Port:** „In the case of web containers, an additional variable $PORT is defined. PORT: Port number your server has to bind on“ (<https://doc.scalingo.com/platform/app/environment>).
- **Lange Verbindungen / Timeouts** (<https://doc.scalingo.com/platform/internals/routing>): „WebSockets, Server-Sent Events (SSE) and long-running connections are fully supported and available by default.“ Die App hat aber anfangs 30 s Zeit für eine Antwort. Danach wächst das Fenster auf 60 s, und es müssen mindestens alle 59 s Daten fließen, sonst kommt `504 Gateway Timeout`. Folgen:
  - Tool-Aufrufe müssen in < 30 s antworten (am Telefon ohnehin nötig; Ninox-Abfrage + Zwischenspeicher im Blick behalten).
  - Mit `Stateless` + `JSONResponse` gibt es keinen lang offenen GET-SSE-Stream, der Leerlauf-Timeout greift also nicht. Ohne diese Optionen würde ein offener GET-Stream nach 59 s ohne Daten abgebrochen; der Client müsste neu verbinden.
- **Header vom Router:** Scalingo fügt `X-Real-IP`, `X-Forwarded-For`, `X-Forwarded-Proto`, `X-Forwarded-Port`, `X-Request-ID` und `X-Request-Start` hinzu (<https://doc.scalingo.com/platform/internals/routing>). Diese Header tauchen also im Log auf und stammen nicht von Placetel. `X-Request-ID` eignet sich zum Verknüpfen von Log-Zeilen.
- HTTP/2 endet am Router, zur App geht HTTP/1.1 (<https://doc.scalingo.com/platform/internals/routing>). Kein Problem für Streamable HTTP.

## Offene Punkte

- Was Placetel beim Aufruf tatsächlich sendet (Header, `_meta`, Protokollversion, „Sitzungskontext“): erst messbar, wenn Placetel das Ausführen im Anruf freischaltet. Bis dahin lassen sich mit „Tools entdecken“ immerhin `initialize`/`tools/list` samt Headern protokollieren.
- Ob eine Gesprächs-ID oder die Rufnummer mitkommt: entscheidet die Frage Werkzeug-Aufruf ↔ Anliegen (siehe #1, „Speicher auf Scalingo“).
