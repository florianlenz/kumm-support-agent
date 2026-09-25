// Wegwerf-Testdienst für die Placetel-Tests (#2, #3): protokolliert jede
// Anfrage und simuliert Foto-Status, Maschinensuche und verzögerte Antworten.
package main

import (
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"
)

type Konfig struct {
	Token        string
	FotoVersuche int
	FotoLeerlauf time.Duration
	ProtokollMax int
}

func konfigAusUmgebung() Konfig {
	k := Konfig{
		Token:        os.Getenv("API_TOKEN"),
		FotoVersuche: 3,
		FotoLeerlauf: 10 * time.Minute,
		ProtokollMax: 1000,
	}
	if v, err := strconv.Atoi(os.Getenv("FOTO_VERSUCHE")); err == nil && v >= 0 {
		k.FotoVersuche = v
	}
	return k
}

// NeuerRouter verdrahtet alle Endpunkte. Alles außer /health braucht das Token.
func NeuerRouter(k Konfig, p *Protokoll) http.Handler {
	d := &Dienst{Foto: NeuerFotoSpeicher(k.FotoVersuche, k.FotoLeerlauf), Anliegen: NeuesAnliegenRegister()}
	geschuetzt := http.NewServeMux()
	geschuetzt.HandleFunc("POST /api/foto-status", d.fotoStatus)
	geschuetzt.HandleFunc("POST /api/foto-status/reset", d.fotoReset)
	geschuetzt.HandleFunc("POST /api/verzoegert/{sekunden}", d.verzoegert)
	geschuetzt.HandleFunc("POST /api/maschine-suchen", d.maschineSuchen)
	geschuetzt.HandleFunc("POST /webhook/inbound", d.inbound)
	geschuetzt.HandleFunc("POST /webhook/nachbearbeitung", d.nachbearbeitung)
	geschuetzt.Handle("/mcp", NeuerMCPHandler(d, p, k.Token))
	geschuetzt.Handle("/protokoll", protokollHandler(p))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		schreibeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})
	mux.Handle("/", ErfordereToken(k.Token, geschuetzt))

	// /health und das Abrufen des Protokolls selbst nicht protokollieren.
	auslassen := func(r *http.Request) bool {
		return r.URL.Path == "/health" || (r.URL.Path == "/protokoll" && r.Method == http.MethodGet)
	}
	return Protokolliere(p, k.Token, auslassen, mux)
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))
	k := konfigAusUmgebung()
	if k.Token == "" {
		slog.Error("API_TOKEN ist nicht gesetzt")
		os.Exit(1)
	}
	port := os.Getenv("PORT") // von Scalingo gesetzt
	if port == "" {
		port = "8080"
	}
	p := NeuesProtokoll(k.ProtokollMax, os.Stdout)
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           NeuerRouter(k, p),
		ReadHeaderTimeout: 10 * time.Second,
	}
	slog.Info("placetel-test lauscht", "port", port, "foto_versuche", k.FotoVersuche)
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("server beendet", "err", err)
		os.Exit(1)
	}
}
