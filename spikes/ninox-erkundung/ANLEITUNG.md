# Anleitung: Ninox-Erkundung (#5)

Dieses Werkzeug liest Kumms Ninox-Datenmodell aus – **nur lesend** (es sendet
ausschließlich GET-Anfragen; alles andere wird im Code blockiert). Es beantwortet
die Fragen aus #5: Wo steht die Maschinennummer, wie hängt die Maschine am
Kunden, welche Kundenfelder gibt es, wie sehen die Maschinennummern aus, gibt
es Dubletten, wie verhält sich die API.

Hintergrund zur API (Quellen, offene Punkte): [NINOX-API.md](NINOX-API.md).

## Datenschutz – bitte zuerst lesen

- **Den API-Schlüssel nie ins Repo, in Issues, PRs oder Chats kopieren.** Nur
  als Umgebungsvariable im eigenen Terminal setzen, im Passwortmanager ablegen.
- **Keine Rohdaten von Kunden** (Namen, Maschinennummern, Adressen) ins Repo
  oder ins Issue. Der Werte-Bericht enthält nur maskierte Muster
  (Buchstabe → `A`, Ziffer → `9`) und Zählungen.
- Alles, was das Werkzeug schreibt, landet in `ausgabe/`. Das Verzeichnis und
  alle `*.json` sind per `.gitignore` ausgeschlossen. Die Datei
  `roh-schema-*.json` enthält nur die Struktur (keine Datensätze), bleibt aber
  trotzdem lokal.
- Vor dem Einfügen ins Issue den Bericht kurz durchsehen. Tabellen- und
  Feldnamen sind unkritisch; Kundennamen dürfen nicht darin stehen.
- Nach der Erkundung den Schlüssel widerrufen, wenn er nicht weiter gebraucht
  wird (oder mit Ablaufdatum anlegen).

## 1. Herausfinden, welches Ninox Kumm nutzt

- **Ninox 3 (klassisch)**: Web-App unter `app.ninox.com` (oder eigene Domain
  `….ninoxdb.de` = Private Cloud). Aufbau: Team → Datenbank → Tabelle.
- **Ninox 4**: Web-App unter `go.ninox.com`. Aufbau: Workspace → Modul → Tabelle.

Im Zweifel: Wer bei Kumm Ninox betreut, fragen oder in die Adresszeile schauen.

## 2. API-Schlüssel anlegen

Möglichst **nur lesend** und **mit Ablaufdatum** (z. B. in 30 Tagen).

### Ninox 3, Public Cloud (`app.ninox.com`)

Muss der Eigentümer des Teams machen.

1. `app.ninox.com` öffnen → Zahnrad oben rechts → **Ninox Settings**
   (oder direkt `https://admin.ninox.com`).
2. Menüpunkt **Integrations** → **New API Key**.
3. Beschreibung eintragen, z. B. „Telefonassistent-Erkundung (Florian)“.
4. Schlüssel kopieren.

Achtung: Laut Ninox gewähren diese Schlüssel **Lese- und Schreibzugriff auf alle
Datenbanken** – ein Nur-Lese-Schlüssel ist in der Public Cloud nicht vorgesehen.
Das Werkzeug schreibt trotzdem nie. Den Schlüssel deshalb besonders sorgfältig
ablegen und nach der Erkundung widerrufen (Papierkorb-Symbol in derselben
Liste).

### Ninox 3, Private Cloud / On-Premises (`….ninoxdb.de`)

Wie oben, aber unter `https://<name>.ninoxdb.de/admin` → **Integrations**. Hier
kann man dem Schlüssel **Rollen** zuweisen. Laut Ninox brauchen alle Zugriffe
außer dem Lesen/Schreiben von Datensätzen die Rolle „admin“ – das Auslesen des
Schemas also vermutlich auch. Wenn ohne „admin“ ein 401/403 kommt, mit „admin“
anlegen.

### Ninox 4 (`go.ninox.com`)

1. Zahnrad **Settings** → unter **WORKSPACE** → **API & integrations**.
2. **Create API key**, Namen eintragen, **Expiration date** setzen.
3. Falls beim Anlegen ein **Scope** gewählt werden kann: nur Lese-Rechte
   (Schema und Datensätze lesen) wählen. Wie die Auswahl genau heißt, ist nicht
   dokumentiert – bitte notieren, was angeboten wird.
4. Schlüssel kopieren.
5. **Workspace-ID** notieren: 12 Zeichen aus Kleinbuchstaben und Ziffern,
   vermutlich in der Adresszeile sichtbar, wenn der Workspace offen ist (nicht
   bestätigt; sonst in der Swagger-Oberfläche unter **API & integrations**
   nachsehen).

## 3. Werkzeug starten

Voraussetzung: Go 1.22 oder neuer. Im Verzeichnis `spikes/ninox-erkundung`:

```sh
cd spikes/ninox-erkundung

# Schlüssel nur für diese Sitzung setzen (führendes Leerzeichen hält ihn
# in zsh/bash aus der History, wenn HIST_IGNORE_SPACE aktiv ist):
 read -rs "NINOX_TOKEN?Ninox-Schlüssel: "; export NINOX_TOKEN   # zsh
# bash:  read -rsp "Ninox-Schlüssel: " NINOX_TOKEN; export NINOX_TOKEN
```

### Schritt A: Datenmodell lesen

Ninox 3 (Public Cloud) – keine weiteren Angaben nötig, es werden alle Teams und
Datenbanken gelesen:

```sh
go run .
```

Ninox 3 Private Cloud: zusätzlich `export NINOX_BASE_URL=https://<name>.ninoxdb.de/v1`.

Ninox 4:

```sh
export NINOX_WORKSPACE_ID=<12 Zeichen>
go run .
```

Optional eingrenzen: `NINOX_TEAM_ID`, `NINOX_DB_ID` (Ninox 3) bzw.
`NINOX_MODULE` (Ninox 4). Alle Optionen: `go run . -h`.

Ergebnis: `ausgabe/schema-<Zeitstempel>.md`. Darin im Abschnitt
„Kandidaten“ nachsehen, welche Tabelle die Maschinen und welches Feld die
Maschinennummer enthält.

### Schritt B: Maschinennummern auswerten (maskiert)

Tabelle und Feld aus Schritt A einsetzen (Name oder ID). `-max 0` liest **alle**
Datensätze – nötig für die Zahl der Maschinen und die Dubletten:

```sh
go run . -tabelle "Maschinen" -feld "Maschinennummer" -max 0
```

Ergebnis: `ausgabe/werte-<Zeitstempel>.md` mit Anzahl, maskierten Formaten,
Längen, Dubletten nach Vereinheitlichung (Punkte, Leerzeichen, Bindestriche
weg, Großschreibung) und Kollisionen der letzten 8 Zeichen.

Ist die Tabelle mehrdeutig (gleicher Name in mehreren Datenbanken), vorher
`NINOX_DB_ID` bzw. `NINOX_MODULE` setzen.

### Wenn etwas nicht klappt

| Meldung | Ursache / Abhilfe |
|---|---|
| `HTTP 401` | Schlüssel falsch oder falsche Variante: Ninox-3-Schlüssel ohne `NINOX_WORKSPACE_ID`, Ninox-4-Schlüssel mit. Private Cloud → `NINOX_BASE_URL`. |
| `HTTP 403` | Schlüssel hat zu wenig Rechte (Rolle/Scope). |
| `HTTP 429` | Zu viele Anfragen – `-pause 1s` angeben. |
| `missing LC_UUID load command` / Abbruch unter macOS | Bekanntes Problem älterer Go-Versionen mit neuem macOS: Go aktualisieren oder `go run -ldflags=-linkmode=external .` verwenden. |
| Feld „nicht gefunden“ | Genauen Namen aus dem Schema-Bericht kopieren, oder die Feld-ID (`A`, `B`, …) nehmen. |

## 4. Was zurück ins Issue #5 soll

Bitte als Kommentar in #5 einfügen (nach dem Durchsehen):

1. Welche Variante: Ninox 3 Public Cloud / Private Cloud / Ninox 4; ob ein
   Nur-Lese-Schlüssel möglich war (und welche Scopes/Rollen es gab).
2. Aus `schema-….md`: die Abschnitte **Verknüpfungen** und **Kandidaten** sowie
   die Tabellen **Maschinen** und **Kunden** (bzw. wie sie heißen). Den Rest
   nur, wenn er kurz ist.
3. Den kompletten `werte-….md` (enthält nur maskierte Muster und Zahlen).
4. Den Abschnitt **API-Verhalten** (Antwortzeiten, Statuscodes, 429 ja/nein).

**Nicht** einfügen: den Schlüssel, `roh-schema-*.json`, echte
Maschinennummern oder Kundennamen.
