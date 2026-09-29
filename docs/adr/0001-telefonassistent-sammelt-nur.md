# Der Telefonassistent sammelt nur, der Abgleich folgt nach dem Anruf

Der Telefonassistent ruft im Gespräch keine eigenen Werkzeuge auf. Er nimmt das Anliegen auf und prüft dabei nichts. Maschine und Kunde werden erst nach dem Anruf ermittelt, aus dem, was Placetel in der Nachbearbeitung liefert. Der Anrufer bekommt nichts auf Basis der gefundenen Maschine; die Angaben sind für den Support. Ein Nachschlagen im Gespräch hätte nur eine Frage gespart, nämlich die nach Name, Betrieb und Ort.

Grundlage sind die Placetel-Tests vom 28.09.2026 (Ticket „Placetel-Test: Fragt der Telefonassistent einen Endpunkt zuverlässig wiederholt ab?“). Sie zeigten:

- Der Telefonassistent ruft Werkzeuge nur in seinem eigenen Gesprächszug auf und fragt nicht von sich aus wiederholt ab.
- Placetel wartet 7 s sicher auf eine Antwort, 10 s nicht mehr.
- Er erfindet Regeln („10 ist nicht erlaubt“) und kündigt Handlungen an, die er nicht ausführt („ich schaue weiter nach“).

Jedes Werkzeug im Gespräch wäre eine Stelle, an der das Gespräch kippen kann.

## Im Gespräch

- **Maschinennummer:** Nur bei Technischem Problem oder Ersatzteil. Er fragt nach den letzten 8 Zeichen, **freiwillig** und **ungeprüft**. Er übernimmt die Angabe so, wie der Anrufer sie nennt, und liest sie einmal zur Bestätigung vor.
- **Name, Betrieb und Ort:** Er fragt **immer** danach. Früher war das nur vorgesehen, wenn der Kunde unbekannt ist.
- **Kein Foto vom Typenschild in Version 1.** Die WhatsApp-Nummer von Kumm hängt an Zendesk, und alles, was in Zendesk landet, ist ausgeschlossen.

## Verworfene Optionen

- **Maschinennummer im Gespräch mit Ninox abgleichen:** Das hätte Werkzeuge im Gespräch, einen Zwischenspeicher und die Zuordnung der Werkzeug-Aufrufe zum Anliegen gebraucht. Dazu kommen die Wartegrenze und kein guter Ausgang bei „nicht gefunden“.
- **Foto per WhatsApp, vom Telefonassistenten wiederholt abgefragt:** scheitert daran, dass er nicht von sich aus wiederholt abfragt.
- **Foto per WhatsApp über die Kumm-Nummer:** Das Foto landet in Zendesk, ausgeschlossen.
- **Eigene zweite WhatsApp-Nummer oder Upload-Link per SMS:** möglich, lohnt sich für Version 1 aber nicht.

## Konsequenzen

- Unser Dienst bekommt alles über die Nachbearbeitung. Ob dort Kategorie, Maschinennummer, Name, Betrieb, Ort und Problem zuverlässig ankommen, entscheidet über die Umsetzung (Ticket „Placetel-Test: Was kommt in der Nachbearbeitung an?“).
- Eine nicht eindeutige oder nicht gefundene Maschinennummer stellt sich erst nach dem Gespräch heraus. Sie wird dann im Anliegen gemeldet, und der Support klärt sie beim Rückruf.
