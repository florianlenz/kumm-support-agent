# Telefonassistent-Tooling

Werkzeuge, die ein bei Placetel betriebener KI-Telefonassistent während eines
laufenden Support-Anrufs bei Kumm aufruft. Der Assistent selbst gehört nicht zu
diesem Projekt — er ist unser Aufrufer, nicht unser Produkt.

## Beteiligte

**Kumm**:
Hersteller und Verkäufer von Güllefässern, dessen Support-Hotline der
Telefonassistent bedient.
_Avoid_: Kunde, Auftraggeber

**Telefonassistent**:
Der KI-Sprachassistent, der bei Placetel konfiguriert ist und den Anruf führt.
_Avoid_: Agent, Bot, Voicebot, Supportagent

**Mitarbeiter**:
Ein Mensch im Support von Kumm, an den der Telefonassistent übergeben kann.
_Avoid_: Agent, Servicemitarbeiter

**Anrufer**:
Die Person am anderen Ende der Leitung. Meist ein Kunde selbst, aber nicht
zwingend (z. B. ein Angestellter des Hofs).
_Avoid_: Caller, Endkunde

**Kunde**:
Ein Landwirt oder Betrieb, der bei Kumm als Kunde geführt wird und Maschinen
besitzt.
_Avoid_: Endkunde, Landwirt (als Fachbegriff)

## Maschine

**Maschine**:
Ein von Kumm verkauftes Gerät (z. B. Güllefass), das einem Kunden zugeordnet ist.
_Avoid_: Fahrzeug, Fass, Gerät

**Maschinennummer**:
Die Kennung einer Maschine, wie sie auf dem Typenschild als
„Fahrgestellnummer“ steht. Ohne festes Format, z. B. `TKKC9901`, `XXX36.1024`
oder nur Ziffern.
_Avoid_: FIN, VIN, Fahrgestellnummer, Seriennummer

**Typenschild**:
Das an der Maschine angebrachte Schild, auf dem die Maschinennummer steht.

## Anruf

**Anliegen**:
Das, was der Telefonassistent aus einem Anruf festhält: worum es geht, wer
anruft und — falls es um eine Maschine geht — welche.
_Avoid_: Ticket, Vorgang, Fall, Rückrufauftrag

**Kategorie**:
Die Einordnung eines Anliegens: Technisches Problem, Ersatzteil oder Sonstiges.
Nur bei Technischem Problem und Ersatzteil wird nach der Maschine gefragt.
