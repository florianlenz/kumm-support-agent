# Placetel-Testdienst: Anleitung

Wegwerf-Dienst für die Tests aus [#2](https://github.com/florianlenz/kumm-support-agent/issues/2)
(Wiederhol-Abfrage, Wartezeit) und [#3](https://github.com/florianlenz/kumm-support-agent/issues/3)
(was kommt an, was steht in der Nachbearbeitung). Kein Produktivcode.

Ablauf: **1** deployen → **2** Placetel einrichten → **3** Testanrufe → **4** Protokoll auswerten.

## Was der Dienst kann

| Endpunkt | Zweck |
|---|---|
| `GET /health` | Lebenszeichen, **ohne** Token |
| `POST /api/foto-status` | Simuliert das Warten auf das Foto vom Typenschild. Die ersten `FOTO_VERSUCHE` (Standard 3) Abfragen: `noch_nicht_da`, danach `erkannt` mit `TKKC9901` / „Testbetrieb Müller, Musterdorf“. Zähler je Anrufer (fester Parameter `anrufer`, sonst `rufnummer`, sonst `anliegen_id`, sonst gemeinsamer Zähler `standard`); nach 10 min Ruhe beginnt er neu. |
| `POST /api/foto-status/reset` | Zähler zurücksetzen (alle, oder `{"schluessel":"tel:+49…"}`) |
| `POST /api/verzoegert/{sekunden}` | Antwortet nach n Sekunden mit einem **Kennwort** (5 → Apfel, 15 → Birne, 25 → Kirsche). Nennt der Telefonassistent das Kennwort, ist die Antwort angekommen. |
| `POST /api/maschine-suchen` | Dummy-Suche über die letzten Zeichen der Maschinennummer (Punkte, Leerzeichen, Striche egal) |
| `/mcp` | MCP-Server (Streamable HTTP, stateless, JSON) mit `foto_status`, `maschine_suchen`, `verzoegert` |
| `POST /webhook/inbound` | Inbound Webhook: liefert `anliegen_id` (4 Ziffern) und `begruessung_hinweis` |
| `POST /webhook/nachbearbeitung` | API-Aufgabe der Nachbearbeitung: nimmt alles an (auch kaputtes JSON), wertet Prüfung 4 und 5 automatisch vor |
| `GET /protokoll` / `DELETE /protokoll` | Alle Anfragen der letzten Zeit (max. 1000, nur im Arbeitsspeicher) / leeren |

Alles außer `/health` braucht das Token: `Authorization: Bearer <API_TOKEN>` **oder** `X-API-Key: <API_TOKEN>`.
Im Protokoll wird das Token maskiert: `***(korrekt)` bzw. `***(falsch, n Zeichen)`.

Testdaten für `maschine_suchen`:

| Maschinennummer | Kunde | Test |
|---|---|---|
| `TKKC9901` | Testbetrieb Müller, Musterdorf | eindeutig |
| `XXX36.1024` | Hof Schneider, Beispielhausen | mit Punkt („36.1024“ oder „X36 1024“) |
| `WK-2019-00457` | Agrar GbR Weber, Probstadt | mit Strichen („1900457“) |
| `AB12345678` und `CD 1234 5678` | Fischer / Wagner | **mehrdeutig** bei „12345678“ |

---

## 1. Scalingo

Der Dienst liegt im Unterordner `spikes/placetel-test/` (eigenes `go.mod`, `Procfile`: `web: placetel-test`).
Scalingo baut aus einem Unterordner, wenn `PROJECT_DIR` gesetzt ist; nur dieser Ordner landet im Image
(<https://doc.scalingo.com/platform/app/monorepo>). Go-Version: `// +scalingo goVersion go1.25` in `go.mod`.

Token erzeugen und merken:

```bash
export TOKEN=$(openssl rand -hex 24); echo $TOKEN
```

Mit der Scalingo-CLI (App-Name nach Belieben, hier `kumm-support-agent`):

```bash
scalingo --region osc-fr1 create kumm-support-agent
scalingo --region osc-fr1 --app kumm-support-agent env-set \
  PROJECT_DIR=spikes/placetel-test \
  API_TOKEN=$TOKEN \
  FOTO_VERSUCHE=3
scalingo --region osc-fr1 --app kumm-support-agent git-show   # zeigt die Git-URL
git remote add scalingo git@ssh.osc-fr1.scalingo.com:kumm-support-agent.git
git push scalingo spike/placetel-test:master
```

Alternativ im Dashboard: App anlegen → *Environment* → `PROJECT_DIR`, `API_TOKEN`, `FOTO_VERSUCHE` setzen → per GitHub-Integration den Branch `spike/placetel-test` deployen.

**Wichtig:** nur **1 Container** (Standard). Protokoll und Zähler liegen im Arbeitsspeicher; Neustart oder Deploy leert sie. Alles steht zusätzlich als JSON-Zeile im Log (`scalingo --app kumm-support-agent logs -f`).

Prüfen:

```bash
export BASE=https://kumm-support-agent.osc-fr1.scalingo.io
curl -s $BASE/health                                   # {"status":"ok"}
curl -s -XPOST $BASE/api/maschine-suchen -H "Authorization: Bearer $TOKEN" -d '{"maschinennummer":"tkkc 9901"}'
curl -s -XPOST $BASE/api/foto-status -H "X-API-Key: $TOKEN" -d '{}'   # 3x noch_nicht_da, 4. Mal erkannt
curl -s -XPOST $BASE/api/foto-status/reset -H "X-API-Key: $TOKEN"
curl -s -XPOST $BASE/mcp -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'
curl -s -XDELETE $BASE/protokoll -H "X-API-Key: $TOKEN"              # vor den echten Tests leeren
```

> **30 s gehen hinter Scalingo nicht:** Der Scalingo-Router bricht ab, wenn die App nicht innerhalb von 30 s antwortet
> (<https://doc.scalingo.com/platform/internals/routing>). Deshalb sind die Varianten 5 / 15 / **25** s. Wer 30 s oder mehr
> testen will, muss den Dienst lokal hinter einem Tunnel (z. B. `cloudflared tunnel --url http://localhost:8080`) laufen lassen;
> der Dienst erlaubt bis 120 s.

---

## 2. Placetel einrichten

Überall `https://kumm-support-agent.osc-fr1.scalingo.io` durch die eigene App-URL ersetzen.
Einen **eigenen Test-Telefonassistenten** nehmen (nicht den späteren echten).

### 2.1 KI-Anweisung

Inhalt aus [`prompts/telefonassistent-test.md`](../../prompts/telefonassistent-test.md) (Abschnitt „Hauptanweisung“) ins Feld **Hauptanweisung** einfügen, Abschnitt „Begrüßungszeile“ ins Feld **Begrüßungszeile**.

### 2.2 API-Anfragen (Voice Wizard → Tab **API-Anfragen**)

Für alle drei gilt:

- **HTTP-Methode:** `POST`
- **API Key:** der Wert von `$TOKEN`
- **Headers:**

```json
{
  "Authorization": "Bearer {{api_key}}",
  "Content-Type": "application/json"
}
```

Die Funktionsnamen haben das Präfix `api_`, damit sie nicht mit den gleichnamigen MCP-Werkzeugen kollidieren.

#### a) Test foto_status

- **Name der API-Anfrage:** `Test foto_status`
- **URL:** `https://kumm-support-agent.osc-fr1.scalingo.io/api/foto-status`
- **Funktionsdefinition:**

```json
{
  "type": "function",
  "name": "api_foto_status",
  "description": "Fragt ab, ob das Foto vom Typenschild schon per WhatsApp angekommen ist. Antwort status=noch_nicht_da: in einigen Sekunden erneut aufrufen. Antwort status=erkannt: enthält maschinennummer und kunde.",
  "parameters": {
    "type": "object",
    "properties": {
      "rufnummer": {
        "type": "string",
        "description": "Rufnummer des Anrufers, falls bekannt"
      },
      "anliegen_id": {
        "type": "string",
        "description": "Die vierstellige Anliegen-Nummer aus der Begrüßung, falls bekannt"
      }
    }
  }
}
```

- **Feste Parameter** (Prüfung 3: wird `%%caller_number%%` ersetzt?):

```json
{
  "anrufer": "%%caller_number%%",
  "anrufer_klammer": "{{caller_number}}"
}
```

- **Vorher sagen:** leer lassen (sonst sagt er bei jeder Abfrage denselben Satz).
- **Bedingung / Wann verwenden:** `Wenn der Anrufer ein Foto vom Typenschild per WhatsApp schickt oder geschickt hat. Wiederholt aufrufen, solange status=noch_nicht_da ist.`

#### b) Test maschine_suchen

- **Name der API-Anfrage:** `Test maschine_suchen`
- **URL:** `https://kumm-support-agent.osc-fr1.scalingo.io/api/maschine-suchen`
- **Funktionsdefinition:**

```json
{
  "type": "function",
  "name": "api_maschine_suchen",
  "description": "Sucht Maschine und Kunde anhand der letzten 8 Zeichen der Maschinennummer vom Typenschild. Buchstaben groß, ohne Leerzeichen. Ergebnis: gefunden, mehrdeutig, nicht_gefunden oder zu_kurz.",
  "parameters": {
    "type": "object",
    "properties": {
      "maschinennummer": {
        "type": "string",
        "description": "Die letzten 8 Zeichen der Maschinennummer, z. B. TKKC9901"
      },
      "anliegen_id": {
        "type": "string",
        "description": "Die vierstellige Anliegen-Nummer aus der Begrüßung, falls bekannt"
      }
    },
    "required": ["maschinennummer"]
  }
}
```

- **Feste Parameter:** `{"anrufer": "%%caller_number%%"}`
- **Vorher sagen:** `Einen Moment, ich schaue nach.`
- **Bedingung / Wann verwenden:** `Wenn der Anrufer die Maschinennummer vom Typenschild nennt.`

#### c) Test warten

- **Name der API-Anfrage:** `Test warten`
- **URL:** `https://kumm-support-agent.osc-fr1.scalingo.io/api/verzoegert/{{sekunden}}`
- **Funktionsdefinition:**

```json
{
  "type": "function",
  "name": "api_warten",
  "description": "Testwerkzeug: antwortet erst nach der angegebenen Anzahl Sekunden und liefert ein Kennwort.",
  "parameters": {
    "type": "object",
    "properties": {
      "sekunden": {
        "type": "integer",
        "enum": [5, 15, 25],
        "description": "Wartezeit in Sekunden: 5, 15 oder 25"
      },
      "anliegen_id": {
        "type": "string",
        "description": "Die vierstellige Anliegen-Nummer aus der Begrüßung, falls bekannt"
      }
    },
    "required": ["sekunden"]
  }
}
```

- **Feste Parameter:** `{"anrufer": "%%caller_number%%"}`
- **Vorher sagen:** `Einen Moment bitte.`
- **Bedingung / Wann verwenden:** `Wenn der Anrufer „Wartetest“ mit einer Zahl sagt.`

Falls `{{sekunden}}` in der URL nicht ersetzt wird (Protokoll zeigt Pfad `/api/verzoegert/%7B%7Bsekunden%7D%7D` o. ä.), stattdessen drei API-Anfragen mit fester URL `/api/verzoegert/5`, `/15`, `/25` anlegen.

### 2.3 MCP-Server (Sidebar → **Tools** → Create tool → **MCP Server**)

- **Name:** `Kumm Test`
- **Server-URL:** `https://kumm-support-agent.osc-fr1.scalingo.io/mcp` (mit `/mcp`!)
- **Authentifizierung:** `Bearer-Token`, Token = `$TOKEN`
  (Alternative zum Gegentest: `Eigener Header`, Name `X-API-Key`, Wert = `$TOKEN`)
- **Sitzungskontext:** wenn die Oberfläche Optionen anbietet, **alles** aktivieren/mitgeben, was angeboten wird, und einen Screenshot der Oberfläche machen (die Doku beschreibt das Feld nicht).
- **Tools entdecken** → `foto_status`, `maschine_suchen`, `verzoegert` auswählen → hinzufügen → im Test-Telefonassistenten im Tab **Tools** aktivieren.

Schon „Tools entdecken“ erzeugt Einträge im Protokoll (`initialize`, `tools/list` mit Headern, `clientInfo`, `protocolVersion`). Das ist der erste Teil von Prüfung 2.

### 2.4 Inbound Webhook (Voice Agent → Tab **Erweitert** → Inbound Webhook → Konfigurieren)

**Optional, derzeit nicht eingerichtet.** Die Anliegen-Nummer ist aus Begrüßung und Prompt entfernt; Prüfung 6 bleibt damit offen.

- **Timeout:** `5000` ms · **Bei Fehler oder Timeout:** `Anruf mit Standardwerten fortsetzen`
- **Methode:** `POST` · **Endpoint:** `https://kumm-support-agent.osc-fr1.scalingo.io/webhook/inbound`
- **Authentifizierung:** `Bearer Token` = `$TOKEN`
- **JSON-Body:**

```json
{
  "caller_number": "{{caller_number}}",
  "called_number": "{{called_number}}",
  "forwarded_from_number": "{{forwarded_from_number}}"
}
```

- **Test ausführen** (z. B. caller_number `+491701234567`), dann **Variablen:**

| Variable | Antwortpfad | Standardwert |
|---|---|---|
| `anliegen_id` | `anliegen_id` | `0000` |
| `begruessung_hinweis` | `begruessung_hinweis` | (leer) |

Die Begrüßungszeile steht in [`prompts/telefonassistent-test.md`](../../prompts/telefonassistent-test.md) (siehe 2.1).

### 2.5 Nachbearbeitung (Voice Agent → Tab **Nachbearbeitung** → Aufgabe hinzufügen → **API**)

- **Methode:** `POST` · **URL:** `https://kumm-support-agent.osc-fr1.scalingo.io/webhook/nachbearbeitung`
- **Authentifizierung:** `Bearer Token` = `$TOKEN`
- **Eigene Extraktionen:**

| Variable | Extraktionsanweisung |
|---|---|
| `maschinennummer` | `Die Maschinennummer, die im Gespräch genannt oder von einem Werkzeug gemeldet wurde, genau wie genannt. Falls keine, leer lassen.` |
| `anliegen_nummer` | `Die vierstellige Anliegen-Nummer, die der Assistent zu Beginn genannt hat. Falls keine, leer lassen.` |

- **Request Body:**

```json
{
  "conversationId": "%%conversationId%%",
  "conversation_link": "%%conversation_link%%",
  "recording_link": "%%recording_link%%",
  "caller_number": "%%caller_number%%",
  "subject": "%%subject%%",
  "summary": "%%summary%%",
  "transcript": "%%transcript%%",
  "maschinennummer": "%%maschinennummer%%",
  "anliegen_nummer": "%%anliegen_nummer%%"
}
```

Der Dienst nimmt den Body auch an, wenn Placetel Anführungszeichen oder Zeilenumbrüche im Transkript nicht maskiert. Ob das JSON gültig war, steht in der Notiz `json_gueltig`. Das ist selbst ein Befund für die spätere E-Mail.

---

## 3. Testanrufe

Vor jedem Anruf:

```bash
curl -s -XPOST $BASE/api/foto-status/reset -H "X-API-Key: $TOKEN"
```

Zeitpunkt jedes Anrufs notieren (für `?seit=`). Pro Anruf **nur eine Art** Werkzeug aktiv haben: entweder die API-Anfragen **oder** die MCP-Tools (im Tools-Tab des Test-Telefonassistenten umschalten).

### Anruf A: Wiederhol-Abfrage (#2, dreimal wiederholen)

Aktiv: API-Anfragen. Sagen:

1. (nach der Begrüßung) „Ich habe ein technisches Problem mit meinem Güllefass.“
2. „Ich schicke Ihnen jetzt ein Foto vom Typenschild per WhatsApp.“
3. Dann **schweigen bzw. nur kurz antworten** („ja“, „okay“) und warten, bis er die Maschinennummer nennt.
4. „Danke, tschüss.“

Erwartet: 4 Aufrufe von `/api/foto-status` im Abstand von einigen Sekunden, danach nennt er `TKKC9901` und „Testbetrieb Müller“.

Variante (optional, Ausdauer): `FOTO_VERSUCHE=10` setzen (`scalingo --app kumm-support-agent env-set FOTO_VERSUCHE=10`, startet neu), noch einmal anrufen. Gibt er nach ca. 2 min sauber auf?

Dieser Anruf liefert nebenbei Prüfung 3 (fester Parameter `anrufer`), 4 (Transkript), 5 (Nachbearbeitung) und 6 (`anliegen_id`).

### Anruf B: Wartezeit (#2)

Aktiv: API-Anfragen. Nacheinander sagen und jeweils die Antwort abwarten:

1. „Wartetest fünf.“ → erwartet Kennwort **Apfel**
2. „Wartetest fünfzehn.“ → **Birne**
3. „Wartetest fünfundzwanzig.“ → **Kirsche**

Mitschreiben: Was sagt/tut er in der Pause? Nennt er das Kennwort oder meldet er einen Fehler/Timeout?

### Anruf C: Maschinensuche (Nebentest)

Aktiv: API-Anfragen. „Die Nummer auf dem Typenschild endet auf X X X drei sechs Punkt eins null zwei vier.“ → Hof Schneider.
Dann: „Und das andere Fass: eins zwei drei vier fünf sechs sieben acht.“ → mehrdeutig, er soll nach dem Betrieb fragen.

### Anruf D: MCP (#3, Prüfung 1 und 2)

Aktiv: nur MCP-Tools. Wie Anruf A, dazu „Wartetest fünf“ und eine Maschinennummer nennen.
Wenn im Protokoll **kein** `tools/call` auftaucht, führt Placetel MCP-Tools im Anruf noch nicht aus (Doku: „in Vorbereitung“).

---

## 4. Auswerten

```bash
# alles der letzten 30 Minuten, kompakt
curl -s -H "Authorization: Bearer $TOKEN" "$BASE/protokoll?seit=30m" \
  | jq '.eintraege[] | {nr, zeit, art, methode, pfad, body, notizen, status, dauer_ms}'

# nur Foto-Abfragen mit Zeitpunkten (Abstände = Abfragerhythmus)
curl -s -H "Authorization: Bearer $TOKEN" "$BASE/protokoll?pfad=/api/foto-status" \
  | jq -r '.eintraege[] | [.zeit, .notizen.foto_abfrage_nr, .antwort.status, .notizen.fester_parameter_anrufer] | @tsv'

# Wartezeit-Tests
curl -s -H "Authorization: Bearer $TOKEN" "$BASE/protokoll?pfad=/api/verzoegert" \
  | jq '.eintraege[] | {zeit, pfad, status, dauer_ms, notizen}'

# MCP: was schickt Placetel mit? (Header, _meta, clientInfo, Argumente)
curl -s -H "Authorization: Bearer $TOKEN" "$BASE/protokoll?pfad=/mcp" \
  | jq '.eintraege[] | select(.art=="mcp") | {zeit, methode, header, body}'

# Inbound Webhook und Nachbearbeitung
curl -s -H "Authorization: Bearer $TOKEN" "$BASE/protokoll?pfad=/webhook" | jq '.eintraege[]'

# ab einem festen Zeitpunkt
curl -s -H "Authorization: Bearer $TOKEN" "$BASE/protokoll?seit=2026-09-26T10:00:00%2B02:00" | jq

# als Datei für das Ticket sichern
curl -s -H "Authorization: Bearer $TOKEN" "$BASE/protokoll" > protokoll-$(date +%F-%H%M).json
```

`notizen` enthält die Vorauswertung des Dienstes. Header wie `X-Request-Id`, `X-Forwarded-For`, `X-Real-Ip` kommen vom Scalingo-Router, nicht von Placetel.

### Checkliste

| Prüfung | Wo schauen | Worauf achten |
|---|---|---|
| **#2 Wiederhol-Abfrage** | `pfad=/api/foto-status`, Anruf A ×3 | Pro Anruf ≥ 4 Aufrufe? Abstand zwischen den Aufrufen (Sekunden)? Nennt er am Ende `TKKC9901`? Verhält er sich am Telefon normal (Füllsätze, keine Endlosschleife, kein Auflegen)? **Erfolg: 3 von 3.** |
| **#2 Wartezeit** | `pfad=/api/verzoegert`, Anruf B | Nennt er Apfel / Birne / Kirsche? `status: 200` und `dauer_ms` ≈ 5000/15000/25000? Notiz `verbindung_vom_aufrufer_getrennt` / `abgebrochen_nach_ms` = Placetel hat früher aufgegeben (Wert = Wartegrenze). Hinweis: Ob der Scalingo-Router einen Abbruch an den Dienst weitergibt, ist nicht sicher. Maßgeblich ist, ob er das Kennwort nennt. |
| **#3.1 MCP im Anruf** | `pfad=/mcp`, `art=mcp`, Anruf D | Gibt es `methode: "tools/call"`? Nur `initialize`/`tools/list` = Ausführung im Anruf noch nicht freigeschaltet. |
| **#3.2 Sitzungskontext** | MCP-Einträge: `header`, `body._meta`, `body.clientInfo`, `body.protocolVersion` | Kommt eine Gesprächs-ID, Rufnummer o. ä. mit (Header wie `X-…`, Schlüssel in `_meta`)? Welcher Client (`clientInfo.name`), welche Protokollversion? |
| **#3.3 `%%caller_number%%` in festen Parametern** | Foto-Einträge, Notiz `fester_parameter_anrufer`; `body.anrufer_klammer` | `ersetzt: +49…` = ja. `NICHT ersetzt: %%caller_number%%` = nein. Wurde stattdessen `{{caller_number}}` ersetzt? |
| **#3.4 Werkzeug-Antworten in `%%transcript%%`** | `/webhook/nachbearbeitung`, `body.transcript`, Notiz `pruefung4_merkmale_im_transkript` | Stehen dort nur die gesprochenen Sätze, oder auch Tool-Aufrufe/-Antworten (`noch_nicht_da`, JSON)? Außerdem in der Placetel-Oberfläche (Anrufverlauf → Gespräch) vergleichen. |
| **#3.5 `%%conversationId%%`** | Notiz `pruefung5_conversationId` | „gefüllt und GLEICH wie im conversation_link“ = ja. „leer“ / „NICHT ersetzt“ = nein. |
| **#3.6 Inbound-Variable später im Gespräch** | `/webhook/inbound` (Notiz `vergebene_anliegen_id`) und Werkzeug-Einträge (Notiz `anliegen_id`) | Nennt er die Nummer in der Begrüßung? Kommt sie bei Werkzeugen an („bekannt aus Inbound Webhook“)? Oder „nicht übergeben“ / „UNBEKANNT“ (dann erfunden)? Zusätzlich in Placetel unter Gespräch → Details → `tec_outputs` nachsehen, ob `{{anliegen_id}}` / `%%anliegen_id%%` in der KI-Anweisung ersetzt wurde. |

Ergebnisse als Kommentar in #2 bzw. #3, Protokoll-Datei anhängen oder die wichtigsten Einträge zitieren.
