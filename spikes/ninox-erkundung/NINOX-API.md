# Ninox-API – Recherche für #5

Stand: 2026-09-25. Quellen sind die Primärdokumentation von Ninox
(docs.ninox.com für Ninox 4, forum.ninox.com/category/docs für Ninox 3) und die
von Ninox ausgelieferte OpenAPI-Spezifikation. **[unbestätigt]** markiert
Aussagen, die ich nicht direkt in einer Primärquelle gefunden habe.

## Zwei APIs: Ninox 3 und Ninox 4

Ninox führt zwei Produktlinien mit getrennter Doku: „Ninox 4“ und das
„klassische“ Ninox 3 ([docs.ninox.com][d-home] verlinkt für Ninox 3 auf
[forum.ninox.com/category/docs][f-docs]). Beide haben eine eigene REST-API.

| | Ninox 3 (klassisch) | Ninox 4 |
|---|---|---|
| Aufbau | Team → Datenbank → Tabelle → Feld | Workspace → Modul → Tabelle → Feld |
| Basis-URL Public Cloud | `https://api.ninox.com/v1` | `https://go.ninox.com/api/v1` **[unbestätigt, abgeleitet]** |
| Private Cloud / On-Premises | `https://{name}.ninoxdb.de/v1` bzw. `.com`, On-Premises auch IP:Port oder eigene Domain | nicht dokumentiert |
| Schlüssel | persönlicher Schlüssel für alle Teams des Nutzers | Workspace-Schlüssel, gilt nur für einen Workspace |
| IDs | Tabellen/Felder `A`, `B`, … `AA` | Namen (`^[a-z0-9_]+$`) |

Quellen: Ninox 3 [Public Cloud][f-public], [Private Cloud/On-Premises][f-private]
(„A Private Cloud domain could be "mycloud.ninoxdb.de" or "mycloud.ninoxdb.com"
while an On-Premises domain might be an IP and port number or a custom domain“);
Ninox 4 [Introduction][d-intro] und die OpenAPI-Spezifikation
[go.ninox.com/api/docs-json][oas] (Pfade beginnen mit `/api/v1/workspace/…`,
`servers` ist leer). Die Basis-URL für Ninox 4 ist daraus abgeleitet: die
Spezifikation wird unter `go.ninox.com` ausgeliefert, und
`GET https://go.ninox.com/api/v1/workspace/<id>` ohne Schlüssel antwortet
mit HTTP 401 (selbst geprüft am 2026-09-25). `GET https://api.ninox.com/v1/teams`
ohne Schlüssel antwortet ebenfalls mit 401.

**Welche Variante Kumm nutzt, ist offen.** Das Werkzeug kann beide.

## Authentifizierung

Beide APIs: Header `Authorization: Bearer <Schlüssel>`.

### Ninox 3

Aus [Introduction to Ninox API][f-intro]:

- Schlüssel anlegen (Workspace-Eigentümer in der Public Cloud bzw. Nutzer mit
  vollem Zugriff auf die Ninox-Einstellungen in der Private Cloud):
  app.ninox.com → Zahnrad → *Ninox Settings* (oder direkt `admin.ninox.com`,
  Private Cloud `[domain].ninoxdb.de/admin`) → *Integrations* → *New API Key*.
- „Your Ninox API keys grant read and write access to all of your databases“ –
  **in der Public Cloud gibt es keinen Nur-Lese-Schlüssel.**
- „In the Private Cloud, roles can also be assigned to an API key at this point.
  All API accesses except those that only read or write at the record level
  require the “admin” role.“ – eine Einschränkung über Rollen gibt es nur in der
  Private Cloud; ob damit echtes Nur-Lesen möglich ist, ist **[unbestätigt]**.
- Löschen/Widerrufen über das Papierkorb-Symbol in derselben Liste.

### Ninox 4

Aus [API and integrations][d-keys] und der [OpenAPI-Spezifikation][oas]:

- Einstellungen (Zahnrad) → unter *WORKSPACE* → *API & integrations* →
  *Create API key*; Name und optional Ablaufdatum angeben.
- Die Schlüsselliste zeigt u. a. eine Spalte **Scope**. Die Spezifikation
  nennt Berechtigungen wie `records:write` (für `POST …/script/exec`) und
  `schema:manage-permissions`; Antwort 403 „Insufficient API key scope“.
  Daraus folgt, dass Schlüssel eingeschränkte Rechte haben können. Ob man in
  der Oberfläche einen **Nur-Lese-Schlüssel** anlegen kann und wie die
  Scopes heißen, ist **[unbestätigt]** – beim Anlegen prüfen.
- Schlüssel gilt nur für den Workspace, in dem er angelegt wurde.

## Endpunkte (nur die lesenden, die wir brauchen)

### Ninox 3 ([Public Cloud][f-public], [Tables, fields, and records][f-tfr])

| Zweck | Anfrage |
|---|---|
| Teams | `GET /teams` → `[{id, name}]` |
| Datenbanken | `GET /teams/{team}/databases` → `[{id, name}]` |
| Datenbankschema | `GET /teams/{team}/databases/{db}` → `{settings, schema:{types…}}` (Header `nx-password` bei geschützten DBs) |
| Tabellen mit Feldern | `GET /teams/{team}/databases/{db}/tables` → `[{id, name, fields:[{id, name, type, …}]}]` |
| Datensätze | `GET …/tables/{tid}/records` → `[{id, sequence, createdAt, modifiedAt, fields:{Feldname: Wert}}]` |
| Datensatz | `GET …/tables/{tid}/records/{rid}` |
| Suche (gleich) | `GET …/tables/{tid}/records?filters={"<Feld-ID>":"Wert"}` |
| Abfrage (Ninox-Skript, lesend) | `GET /teams/{team}/databases/{db}/query?query=<Skript>` oder `POST …/query` mit `{"query": "…"}` – laut Doku „read-only query“ |
| Änderungen | `GET …/databases/{db}/changes?sinceSq={n}`, `GET …/tables/{tid}/changes?sinceSq={n}` |

Parameter für `…/records` ([Tables, fields, and records][f-tfr]): `page`
(Standard 0), `perPage` (Standard 100), `order`, `desc`, `new`, `updated`,
`sinceId`, `sinceSq`, `filters` (JSON-Text, Feld-IDs als Schlüssel), `ids`,
`choiceStyle`. Ein Höchstwert für `perPage` ist nicht dokumentiert
**[unbestätigt]**; das Werkzeug nutzt 500.

Feldtypen laut Doku: text, number, date, datetime, timeinterval, time,
appointment, boolean, choice, url, email, phone, location, html. **Verknüpfungen
fehlen in dieser Liste.** Laut Community liefert `…/records` bei Verknüpfungen
die ID(s) des verknüpften Datensatzes ([Forum][f-rel]). Wie Verknüpfungen im
Schema heißen (vermutlich `base: "ref"`/`"rev"` mit `refTypeId`) ist
**[unbestätigt]** – das Werkzeug zeigt daher alle Schema-Angaben eines Feldes
an.

`filters` vergleicht nur auf Gleichheit. Für „endet auf“/„enthält“ braucht es
die Abfrage-Schnittstelle mit Ninox-Skript, z. B.
`(select Maschinen where contains(Maschinennummer, "1024")).Id`
**[Skript-Syntax unbestätigt]**. Für unseren Dienst ist ohnehin geplant, Ninox
zwischenzuspeichern und dort zu suchen (#1), daher reicht `…/records` mit
Paging.

### Ninox 4 ([OpenAPI][oas], [Swagger][swagger])

| Zweck | Anfrage |
|---|---|
| Workspace inkl. Module | `GET /workspace/{ws}` |
| Module | `GET /workspace/{ws}/modules?limit&offset` |
| Tabellen | `GET /workspace/{ws}/modules/{modul}/tables?limit&offset` |
| Felder | `GET /workspace/{ws}/modules/{modul}/tables/{tabelle}/fields?limit&offset` |
| Datensätze | `GET …/tables/{tabelle}/records?limit(1–100)&offset&fields&filter&sort` → `{data:[{id, values}], page_info:{has_more, limit, offset}}` |
| Datensatz | `GET …/tables/{tabelle}/record/{id}` |
| Änderungen | `GET …/records/changes`, `GET /workspace/{ws}/schema/changes` |
| Skript | `POST /workspace/{ws}/script/exec` – **schreibend**, braucht `records:write` |

Feldtypen (Enum der Spezifikation): any, appointment, boolean, choice, color,
date, dchoice, dmulti, file, function, html, icon, lambda, multi, number, react,
**reference** (mit `refTableName`), **reverse**, rowId, string, styled, time,
timeinterval, timestamp, unknown, user, void. `filter` ist ein JSON-Objekt
(`{"email": "…"}`, Gleichheit); `filters` ist der veraltete Name aus Ninox 3.
Einen lesenden Skript-Endpunkt gibt es in Ninox 4 **nicht**.

## Rate-Limits und Antwortzeiten

- Weder die Ninox-3- noch die Ninox-4-Doku noch die OpenAPI-Spezifikation nennen
  Rate-Limits oder HTTP 429.
- Ein Ninox-Mitarbeiter schrieb im Forum: „We have introduced a limit on the
  number of parallel API calls. Please make sure to wait for the answer before
  sending the next request.“ ([Forum-Thread zu 429][f-429], ca. 2020). Konkrete
  Zahlen gibt es dort nicht.
- Die Ninox-4-Einstellungen verweisen für API-Nutzung auf *Subscriptions and
  usage* ([API and integrations][d-keys]) – möglicherweise gibt es ein
  Kontingent je Abo **[unbestätigt]**.
- Das Werkzeug stellt deshalb nur **eine Anfrage nach der anderen**, wartet
  250 ms dazwischen, wiederholt bei 429/502/503/504 (mit `Retry-After`) und
  misst Antwortzeiten und Statuscodes (Abschnitt „API-Verhalten“ im Bericht).

## Folgen für den Dienst (vorläufig)

- Suche nach den letzten 8 Zeichen: nicht über die API filtern, sondern alle
  Maschinen (nur benötigte Felder) laden und lokal vereinheitlicht suchen –
  passt zum geplanten Zwischenspeicher.
- Aktualisieren: Ninox 3 `…/changes?sinceSq=`, Ninox 4 `…/records/changes`.
- Ninox 3 Public Cloud: Schlüssel hat Schreibrechte → sicher ablegen, im Dienst
  nur GET verwenden.

[d-home]: https://docs.ninox.com/
[d-intro]: https://docs.ninox.com/ninox-api/api-reference/introduction-to-ninox-public-api
[d-keys]: https://docs.ninox.com/builder-hub/manage-your-organization-and-workspace/organize-your-workspace/api-and-integrations
[oas]: https://go.ninox.com/api/docs-json
[swagger]: https://go.ninox.com/api/docs
[f-docs]: https://forum.ninox.com/category/docs
[f-intro]: https://forum.ninox.com/t/83yzlg7/introduction-to-ninox-api
[f-public]: https://forum.ninox.com/t/35yzp89/api-endpoints-for-public-cloud
[f-private]: https://forum.ninox.com/t/x2yzfmf/api-endpoints-for-private-cloudon-premises
[f-tfr]: https://forum.ninox.com/t/x2yzljq/tables-fields-and-records
[f-rel]: https://forum.ninox.com/t/p8hrtvf
[f-429]: https://forum.ninox.com/t/35hr60f
