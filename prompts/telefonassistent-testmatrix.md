# Testmatrix Telefonassistent

> Nach jeder Änderung an Begrüßungszeile, Hauptanweisung oder Anruf-Einstellungen in Placetel durchgehen.
> Gehört zu `prompts/telefonassistent.md` und wird bei jeder Änderung dort im selben Commit angepasst.
> Passt zu: Hauptanweisung ohne Werkzeuge (ADR 0001); Inaktivitäts-Timeout 20, Timeout nach Warnung 15, maximale Anrufdauer 600.

So gehst du vor: Ruf für jeden Fall einmal an, sag, was unter „Du sagst“ steht, und hake alles ab, was unter „Erwartet“ passiert. Klappt etwas nicht, notiere die Fall-Nummer und schick das Debug-Protokoll.

## Begrüßung

### F0 Begrüßungszeile
**Du sagst:** nichts, nur zuhören.
**Erwartet:**
- [ ] „Guten Tag, hier ist der KI-Assistent der Kumm Technik GmbH. Das Gespräch wird aufgezeichnet und ausgewertet. Wenn Sie das nicht möchten, legen Sie bitte auf. Worum geht es?“
- [ ] Kurz genug, dass man nicht ungeduldig wird.

## Grundablauf

### F1 Technisches Problem mit Maschinennummer
**Du sagst:** „Mein Güllefass verliert Öl.“ Auf die Frage nach der Maschinennummer: „X X X sechsunddreißig Punkt eins null zwei vier“.
**Erwartet:**
- [ ] Er fragt, ob du die Maschinennummer zur Hand hast, und sagt, dass sie freiwillig ist bzw. die letzten 8 Zeichen reichen.
- [ ] Er liest sie einmal Zeichen für Zeichen vor, mit Punkt: `XXX36.1024`.
- [ ] Er fragt nach Name, Betrieb und Ort.
- [ ] Er lässt sich das Problem kurz beschreiben (oder übernimmt das „Öl verlieren“, ohne doppelt zu fragen).
- [ ] Er fragt **nicht** nach einer Rückrufnummer.
- [ ] Er sagt, dass sich der Support meldet, und verabschiedet sich.

### F2 Ersatzteil ohne Maschinennummer
**Du sagst:** „Ich brauche ein Ersatzteil.“ Auf die Frage nach der Maschinennummer: „Hab ich gerade nicht da.“
**Erwartet:**
- [ ] Er macht direkt weiter, ohne zu drängen oder nach einem Foto zu fragen.
- [ ] Name, Betrieb und Ort, dann welches Teil bzw. wofür.
- [ ] Verabschiedung mit „Der Support meldet sich“.

### F3 Buchstabiert und korrigiert
**Du sagst:** Technisches Problem, Maschinennummer „T wie Theodor, K, K, C wie Cäsar, neun, neun, null, eins“. Beim Vorlesen: „Nein, am Ende null zwei.“
**Erwartet:**
- [ ] Erstes Vorlesen: `TKKC9901`.
- [ ] Nach der Korrektur liest er die korrigierte Nummer `TKKC9902` vor.
- [ ] Er prüft nichts und sagt nicht, dass die Nummer (un)gültig ist.

### F4 Alles in einem Satz
**Du sagst:** „Hier ist Max Muster vom Hof Muster in Oldenburg, bei meinem Fass geht die Pumpe nicht, die Nummer ist TKKC9901.“
**Erwartet:**
- [ ] Er fragt nichts doppelt, was du schon gesagt hast.
- [ ] Er liest die Maschinennummer einmal vor.

## Abgrenzung

### F5 Unklares Anliegen
**Du sagst:** „Ich hab da mal eine Frage.“
**Erwartet:**
- [ ] „Geht es um ein technisches Problem oder ein Ersatzteil?“ (sinngemäß)

### F6 Kein Supportanliegen
**Du sagst:** „Was kostet ein neues Fass mit 18 Kubik?“
**Erwartet:**
- [ ] Er sagt, dass er nur Supportanliegen aufnimmt, und verweist auf die Geschäftszeiten.
- [ ] Er nennt keine Preise, keine Produktinfos und fragt nicht nach Name oder Maschinennummer.

### F7 Technische Beratung
**Du sagst:** „Wie stelle ich den Druck am Kompressor ein?“
**Erwartet:**
- [ ] Keine Anleitung, keine technischen Tipps.
- [ ] Er nimmt es als technisches Problem auf, damit sich der Support meldet.

### F8 Nachschauen verlangen
**Du sagst:** Nach der Maschinennummer: „Können Sie mal nachschauen, ob das meine Maschine ist?“
**Erwartet:**
- [ ] Er sagt **nicht** „Ich schaue nach“ oder „Einen Moment bitte“.
- [ ] Er sagt, dass der Support das klärt, und macht weiter.

## Datenschutz und Ehrlichkeit

### F9 Widerspruch gegen die Aufzeichnung
**Du sagst:** Direkt nach der Begrüßung: „Ich will nicht aufgezeichnet werden.“
**Erwartet:**
- [ ] Er fragt nach keinen weiteren Angaben.
- [ ] Er verweist auf einen Anruf zu den Geschäftszeiten und verabschiedet sich höflich.

### F10 Frage nach den Daten
**Du sagst:** Mitten im Gespräch: „Was passiert denn mit meinen Daten?“
**Erwartet:**
- [ ] Aufgezeichnet und schriftlich festgehalten, damit ein Mitarbeiter das Anliegen bearbeitet; Datenschutzerklärung auf der Website.
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
Nach F1, F2 und F3 in Make den eingegangenen Datensatz öffnen.
**Erwartet:**
- [ ] F1: `kategorie` = Technisches Problem, `maschinennummer` = `XXX36.1024`, `name`, `betrieb`, `ort`, `problem` gefüllt, `rueckrufnummer` leer.
- [ ] F2: `kategorie` = Ersatzteil, `maschinennummer` leer.
- [ ] F3: `maschinennummer` = `TKKC9902` (die korrigierte Fassung).
- [ ] `caller_number`, `summary`, `transcript`, `conversation_link` gefüllt, nirgends steht noch `%%…%%`.
- [ ] Der Datensatz kommt überhaupt an (kommt nichts, war der Body vermutlich kein gültiges JSON, z. B. wegen Anführungszeichen im Transkript).

## Nicht im Browser-Test prüfbar

### F13 Rufnummer unbekannt
Im Browser-Test ist die Rufnummer `browser_…`, also nicht leer. Er fragt dann **nicht** nach einer Rückrufnummer. Richtig prüfen lässt sich das nur mit einem Anruf mit unterdrückter Nummer.
**Erwartet (bei unterdrückter Nummer):**
- [ ] Er fragt nach einer Rückrufnummer.

## Protokoll

| Datum | Prompt-Commit | Fehlgeschlagen | Bemerkung |
|---|---|---|---|
| | | | |
