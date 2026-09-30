## Agent skills

### Issue tracker

Issues are tracked as GitHub issues in `florianlenz/kumm-support-agent`, via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Domain docs

Single-context: `CONTEXT.md` and `docs/adr/` at the repo root. See `docs/agents/domain.md`.

## Telefonassistent-Prompt

`prompts/telefonassistent.md` ist die einzige Quelle für das Verhalten des Telefonassistenten; Placetel bekommt ihren Inhalt per Kopie. Jede Verhaltensänderung (Begrüßungszeile, Hauptanweisung, Nachbearbeitung) landet zuerst dort, erst dann in Placetel.

Dazu gehört die Testmatrix `prompts/telefonassistent-testmatrix.md`, die der Nutzer nach jeder Änderung in Placetel von Hand durchgeht. Ändert sich Begrüßungszeile, Hauptanweisung oder eine Anruf-Einstellung, passe die Matrix im selben Commit an: neue Regeln bekommen einen Fall, gestrichene Regeln verlieren ihren, geänderte Erwartungen werden umgeschrieben.
