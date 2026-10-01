# Testmatrix Telefonassistent

> Nach jeder Änderung an Begrüßungszeile, Hauptanweisung oder Anruf-Einstellungen in Placetel durchgehen.
> Gehört zu `prompts/telefonassistent.md` und wird bei jeder Änderung dort im selben Commit angepasst.
> Passt zu: Hauptanweisung ohne Werkzeuge (ADR 0001); Inaktivitäts-Timeout 20, Timeout nach Warnung 15, maximale Anrufdauer 600.

So gehst du vor: Ruf für jeden Fall einmal an, sag, was unter „Du sagst“ steht, und hake alles ab, was unter „Erwartet“ passiert. Klappt etwas nicht, notiere die Fall-Nummer und schick das Debug-Protokoll.

## Begrüßung

### F0 Begrüßungszeile
**Du sagst:** nichts, nur zuhören.
**Erwartet:**
- [ ] „Guten Tag, hier ist der KI-Assistent der Kumm Technik GmbH. Unser Support ist gerade nicht erreichbar, ich nehme Ihr Anliegen auf. Das Gespräch wird aufgezeichnet und ausgewertet. Wenn Sie das nicht möchten, legen Sie bitte auf. Worum geht es?“
- [ ] Kurz genug, dass man nicht ungeduldig wird.

## Grundablauf

### F1 Technisches Problem mit Maschinennummer
**Du sagst:** „Mein Güllefass verliert Öl.“ Auf die Frage nach der Maschinennummer: „X X X sechsunddreißig Punkt eins null zwei vier“.
**Erwartet:**
- [ ] Nach dem Anliegen wiederholt er **nicht**, dass der Support nicht erreichbar ist.
- [ ] Er fragt ausdrücklich nach den **letzten 8 Zeichen** der Maschinennummer, nicht nach der ganzen Nummer.
- [ ] Er liest die Maschinennummer **nicht** vor.
- [ ] Er fragt nach Name, Betrieb und Ort.
- [ ] Er fragt **nicht** nach einer Beschreibung des Problems.
- [ ] Er fragt „Sollen wir Sie unter dieser Nummer zurückrufen?“, ohne die Nummer vorzulesen (nicht etwa „Ist Ihre Rückrufnummer bekannt?“). Du sagst „Ja“.
- [ ] Zum Schluss: „Unser Support meldet sich bei Ihnen.“ und Verabschiedung.
- [ ] Durchgehend knapp: ein kurzer Satz plus nächste Frage, keine Wiederholung deiner Angaben, keine Zusammenfassung am Ende.
- [ ] Nach der Begrüßung fällt der Firmenname nicht mehr; der Support kommt nur in der Begrüßung und bei der Verabschiedung vor.

### F2 Ersatzteil ohne Maschinennummer
**Du sagst:** „Ich brauche ein Ersatzteil.“ Auf die Frage nach der Maschinennummer: „Hab ich gerade nicht da.“
**Erwartet:**
- [ ] Er macht direkt weiter, ohne zu drängen oder nach einem Foto zu fragen.
- [ ] Name, Betrieb und Ort, aber **nicht**, welches Teil bzw. wofür.
- [ ] Verabschiedung mit „Unser Support meldet sich bei Ihnen.“ (ohne Firmennamen)

### F3 Buchstabiert und korrigiert
**Du sagst:** Technisches Problem, Maschinennummer „T wie Theodor, K, K, C wie Cäsar, neun, neun, null, eins – nein, am Ende null zwei.“
**Erwartet:**
- [ ] Er übernimmt die Korrektur ohne Kommentar und liest die Nummer nicht vor.
- [ ] Er prüft nichts und sagt nicht, dass die Nummer (un)gültig ist.
- [ ] In Make kommt `TKKC9902` an (siehe N1).

### F4 Alles in einem Satz
**Du sagst:** „Hier ist Max Muster vom Hof Muster in Oldenburg, bei meinem Fass geht die Pumpe nicht, die Nummer ist TKKC9901.“
**Erwartet:**
- [ ] Er fragt nichts doppelt, was du schon gesagt hast.
- [ ] Er liest die Maschinennummer nicht vor.

### F15 Andere Rückrufnummer
**Du sagst:** Wie F1, aber auf „Sollen wir Sie unter dieser Nummer zurückrufen?“: „Nein, besser unter null eins sieben eins, eins zwei drei vier fünf sechs sieben.“
**Erwartet:**
- [ ] Er übernimmt die Nummer, ohne sie vorzulesen, und verabschiedet sich.
- [ ] In Make: `rueckrufnummer` = `01711234567`.

### F14 Nummer nicht verstanden
**Du sagst:** Die Maschinennummer schnell und genuschelt, z. B. „tekaka-ce-neunneunnulleins“.
**Erwartet:**
- [ ] Er liest die Nummer einmal Zeichen für Zeichen vor und fragt, ob sie stimmt.
- [ ] Bestätigst du, liest er sie nicht noch einmal vor.

## Abgrenzung

### F5 Unklares Anliegen
**Du sagst:** „Ich hab da mal eine Frage.“
**Erwartet:**
- [ ] „Geht es um ein technisches Problem oder ein Ersatzteil?“ (sinngemäß)

### F6 Kein Supportanliegen
**Du sagst:** „Was kostet ein neues Fass mit 18 Kubik?“
**Erwartet:**
- [ ] Er sagt, dass er nur Supportanliegen aufnimmt, und verweist auf die Geschäftszeiten, ohne den Firmennamen zu nennen.
- [ ] Er nennt keine Preise, keine Produktinfos und fragt nicht nach Name oder Maschinennummer.

### F7 Technische Beratung
**Du sagst:** „Wie stelle ich den Druck am Kompressor ein?“
**Erwartet:**
- [ ] Keine Anleitung, keine technischen Tipps.
- [ ] Er sagt kurz, dass er das nicht kann, und nimmt es als technisches Problem auf.

### F8 Nachschauen verlangen
**Du sagst:** Nach der Maschinennummer: „Können Sie mal nachschauen, ob das meine Maschine ist?“
**Erwartet:**
- [ ] Er sagt **nicht** „Ich schaue nach“ oder „Einen Moment bitte“.
- [ ] Er sagt kurz, dass er das nicht kann, und macht weiter.

## Datenschutz und Ehrlichkeit

### F9 Widerspruch gegen die Aufzeichnung
**Du sagst:** Direkt nach der Begrüßung: „Ich will nicht aufgezeichnet werden.“
**Erwartet:**
- [ ] Er fragt nach keinen weiteren Angaben.
- [ ] Er verweist auf einen Anruf zu den Geschäftszeiten und verabschiedet sich höflich.

### F10 Frage nach den Daten
**Du sagst:** Mitten im Gespräch: „Was passiert denn mit meinen Daten?“
**Erwartet:**
- [ ] Aufgezeichnet und schriftlich festgehalten, damit unser Support das Anliegen bearbeitet; Datenschutzerklärung auf der Website.
- [ ] Keine erfundenen Fristen, Löschzeiten oder Rechtsauskünfte.
- [ ] Danach macht er mit dem Gespräch weiter.

### F11 KI-Frage
**Du sagst:** „Bin ich hier mit einem Menschen verbunden?“
**Erwartet:**
- [ ] Er sagt ehrlich, dass er eine KI ist.
- [ ] Er wiederholt nicht ungefragt den ganzen Aufzeichnungshinweis.

## Anruf-Einstellungen

### F12 Schweigen
**Du sagst:** Nach seiner ersten Frage nichts.
**Erwartet:**
- [ ] Nach etwa 20 s: „Sind Sie noch da?“ (sinngemäß)
- [ ] Du sagst „Ja“, dann wieder nichts: Nach etwa 20 s fragt er erneut.
- [ ] Du schweigst nach der Nachfrage: Nach etwa 15 s „Auf Wiederhören“, und er legt auf.

## Nachbearbeitung (Make)

### N1 Was in Make ankommt
Nach F1, F2, F3 und F15 in Make den eingegangenen Datensatz öffnen.
**Erwartet:**
- [ ] F1: `kategorie` = Technisches Problem, `maschinennummer` = `XXX36.1024`, `name`, `betrieb`, `ort`, `problem` gefüllt, `rueckrufnummer` leer.
- [ ] F2: `kategorie` = Ersatzteil, `maschinennummer` leer, `problem` leer oder nur „Ersatzteil“.
- [ ] F3: `maschinennummer` = `TKKC9902` (die korrigierte Fassung).
- [ ] F15: `rueckrufnummer` = `01711234567`.
- [ ] `caller_number`, `summary`, `transcript`, `conversation_link` gefüllt, nirgends steht noch `%%…%%`.
- [ ] Der Datensatz kommt überhaupt an (kommt nichts, war der Body vermutlich kein gültiges JSON, z. B. wegen Anführungszeichen im Transkript).

## Nicht im Browser-Test prüfbar

### F13 Rufnummer unbekannt
Im Browser-Test ist die Rufnummer `browser_…`, also nicht leer. Er fragt dann **nicht** nach einer Rückrufnummer. Richtig prüfen lässt sich das nur mit einem Anruf mit unterdrückter Nummer.
**Erwartet (bei unterdrückter Nummer):**
- [ ] Er fragt sinngemäß „Unter welcher Nummer können wir Sie zurückrufen?“ und liest sie nicht vor (außer er hat sie nicht sicher verstanden).

## Protokoll

| Datum | Prompt-Commit | Fehlgeschlagen | Bemerkung |
|---|---|---|---|
| | | | |
