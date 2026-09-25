package main

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

func tokenGleich(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// tokenAusAnfrage liest das Token aus "Authorization: Bearer <Token>" oder "X-API-Key: <Token>".
func tokenAusAnfrage(r *http.Request) (bearer, apiKey string) {
	if a := strings.TrimSpace(r.Header.Get("Authorization")); len(a) >= 7 && strings.EqualFold(a[:7], "bearer ") {
		bearer = strings.TrimSpace(a[7:])
	}
	apiKey = strings.TrimSpace(r.Header.Get("X-API-Key"))
	return bearer, apiKey
}

// ErfordereToken lässt nur Anfragen mit korrektem Token durch
// (Bearer oder X-API-Key, eines von beiden reicht).
func ErfordereToken(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bearer, apiKey := tokenAusAnfrage(r)
		ok := token != "" && (tokenGleich(bearer, token) || tokenGleich(apiKey, token))
		if !ok {
			notiz(r.Context(), "auth", "abgelehnt")
			w.Header().Set("WWW-Authenticate", `Bearer realm="placetel-test"`)
			schreibeJSON(w, http.StatusUnauthorized, map[string]any{"fehler": "nicht autorisiert"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
