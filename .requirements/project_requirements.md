# coachly — Projektanforderungen

> Strukturiertes Anforderungsdokument. Jede Anforderung hat eine stabile ID (z. B. `FR-I-03`).
> Beim Hinzufügen neuer Anforderungen: bestehende IDs **nie** ändern, neue IDs am Ende der jeweiligen Gruppe vergeben, Eintrag im Änderungsprotokoll (§9) ergänzen.

---

## 1. Überblick

`coachly` ist eine schlanke Desktop-Anwendung zur Verwaltung von Coaching-Athleten und zur monatlichen Rechnungsstellung. Die App wird von Freunden heruntergeladen und per Doppelklick gestartet — ohne Installation eines Laufzeitstacks. Alle Daten werden lokal verschlüsselt gespeichert; der Zugriff erfolgt über ein Master-Passwort. Rechnungen werden als PDFs erzeugt, optional per SMTP versendet, und können vor dem Versand in einer Vorschau geprüft werden.

**Ursprünglicher Kontext (Spirit):** Ein Coach möchte seine Athleten so übersichtlich wie eine Tabelle pflegen, aber mit mehr Funktionalität, und am Monatsende für alle aktiven Athleten Rechnungen mit fortlaufender Nummer und einem „Coaches Tipp" erzeugen. Mid-Monat-Starts werden anteilig berechnet.

---

## 2. Goals & Non-Goals

**Goals**
- Eine einzige ausführbare Datei pro Betriebssystem (Mac primär, Windows sekundär).
- Doppelklick öffnet automatisch den Standard-Browser auf die App.
- Keine Cloud, keine externen Abhängigkeiten zur Laufzeit. Volle Offline-Fähigkeit.
- Datenschutz first: lokal verschlüsselt, Master-Passwort.

---

## 3. Glossar

| Begriff | Bedeutung |
|---|---|
| **Finanzamt** | Zuständige Steuerbehörde (z. B. „Finanzamt Innsbruck"). Wird auf der Rechnung genannt. |
| **Steuernummer** | Vom Finanzamt vergebene Nummer des Coaches. |
| **UID / USt-ID** | Umsatzsteuer-Identifikationsnummer (Österreich: ATU…). |
| **USt** | Umsatzsteuer; v1 fix 20 %. |
| **Coaches Tipp** | Monatlicher Hinweistext, der auf jeder Rechnung über der Position erscheint. |
| **Stammdaten** | Master-Daten von Coach und Finanzamt (IBAN, BIC, Adresse, USt-ID, …). |
| **Pro-rata** | Anteilige Berechnung bei Coaching-Start mitten im Monat. |

---

## 4. Funktionale Anforderungen

### 4.1 Athleten (`FR-A-*`)

- **FR-A-01** — Athleten anlegen, bearbeiten, löschen.
- **FR-A-02** — Pflichtattribute pro Athlet: Vorname, Nachname, Adresse (Straße, PLZ, Ort, Land), monatliche Gebühr in EUR, Start-Datum, End-Datum (optional, leer = aktiv).
- **FR-A-03** — Liste filterbar (nach Name, Status aktiv/inaktiv, Startdatum, Gebühr).
- **FR-A-04** — Filter benannt speichern und wiederverwenden („Aktiv 2026").
- **FR-A-05** — Beim Coaching-Start mid-month wird der erste Monatsbeitrag anteilig berechnet (Tagesbasis: verbleibende Tage / Gesamttage im Monat).
- **FR-A-06** — Athleten können per CSV-Datei in einem Batch importiert werden. Spalten: `id, first_name, last_name, street, postal_code, city, country, monthly_fee, start_date, end_date, email, notes` (Komma oder Semikolon als Trennzeichen). Vor dem Speichern zeigt eine Vorschau neue, unveränderte, konfliktbehaftete (gleiche ID, andere Daten) und fehlerhafte Zeilen; bei Konflikten entscheidet der Nutzer pro Zeile, ob die neuen Daten übernommen werden.

### 4.2 Rechnungen (`FR-I-*`)

- **FR-I-01** — Rechnungen werden monatlich für alle im Zielmonat aktiven Athleten erzeugt.
- **FR-I-02** — Der Nutzer wählt den Monat, für den Rechnungen erzeugt werden.
- **FR-I-03** — Rechnungsnummern sind fortlaufend und werden nie wiederverwendet. Ausnahme: beim expliziten „Mit selber Rechnungsnummer neu erstellen"-Modus (FR-I-09) wird die Nummer einer **gelöschten editierbaren** Rechnung im selben Vorgang wiederverwendet — die Sequenz bleibt damit nach außen lückenlos.
- **FR-I-04** — Vor dem endgültigen Erzeugen muss der Nutzer alle Rechnungen in einer **Vorschau** durchklicken können.
- **FR-I-05** — Erst nach Bestätigung werden Rechnungsnummern „verbraucht" und PDFs geschrieben.
- **FR-I-06** — Eine Rechnung enthält genau **eine** Positionszeile (= ein Monat). Bei Mid-Month-Start wird der Pro-rata-Betrag verwendet.
- **FR-I-07** — Rechnungs-Lifecycle: **Entwurf → Erstellt → Versendet → Bezahlt**. Statuswechsel `Versendet` und `Bezahlt` sind „geschützt": eine Rechnung in diesem Zustand darf weder gelöscht noch überschrieben werden. Statuswechsel rückwärts (`Versendet → Erstellt`, `Bezahlt → Versendet`) ist über die UI möglich, falls der Coach sich vertan hat.
- **FR-I-08** — **Einzelrechnung**: Neben dem Monats-Batch kann eine einzelne Rechnung für einen einzelnen Athleten erzeugt werden (Athlet + Monat + Tipp). Es gilt dieselbe Pro-rata-Logik wie für den Batch.
- **FR-I-09** — **Konflikt-Modus „Mit selber Rechnungsnummer neu erstellen"**: Wenn für einen Monat bereits Rechnungen existieren, kann der Nutzer wählen, alle editierbaren Rechnungen zu löschen und unter ihrer **ursprünglichen Nummer** neu zu erstellen. Geschützte Rechnungen (Versendet / Bezahlt) bleiben unverändert und werden im UI als „übersprungen" angezeigt. Der bestehende Modus „Alle überschreiben" verwendet weiterhin neue Nummern, lässt geschützte Rechnungen aber ebenfalls unangetastet.

### 4.3 PDF-Ausgabe (`FR-P-*`)

- **FR-P-01** — Jede Rechnung ist genau **eine** Seite.
- **FR-P-02** — Pflichtfelder im Kopfbereich: Empfänger (Athlet), Absender (Coach), Finanzamt, Datum, Rechnungsnummer, Steuernummer, UID.
- **FR-P-03** — Die Coaches-Tipp-Zeile erscheint **vor** der Positionszeile.
- **FR-P-04** — Positionszeile-Spalten: **Zeitraum | Beschreibung | Kosten in Euro (inkl. 20 % USt)**.
- **FR-P-05** — Darunter „Total".
- **FR-P-06** — Nach den Positionen folgt der Text: *„Bitte überweise den oben genannten Betrag binnen 10 Tagen auf das unten genannte Konto."*
- **FR-P-07** — Danach Bank-Block: **Bank, Kontoinhaber, IBAN, BIC**.
- **FR-P-08** — Abschluss: Danke-Zeile + Grußformel.
- **FR-P-09** — Dateiname: `{firstname}_{month}_{year}_{invoice_number}.pdf` (z. B. `max_05_2026_2026042.pdf`). Die Rechnungsnummer am Ende garantiert Eindeutigkeit, falls zwei Athleten denselben Vornamen haben.
- **FR-P-10** — PDFs werden standardmäßig in einem festen Ordner unterhalb des Anwendungs-Datenverzeichnisses abgelegt. Optional kann in den Einstellungen ein abweichender absoluter Pfad konfiguriert werden; ist dieser gesetzt, werden alle neu erzeugten PDFs dort gespeichert. Bereits zuvor erzeugte PDFs bleiben an ihrem Ursprungsort und werden weiterhin über den in der Rechnung gespeicherten Pfad geöffnet.

### 4.4 Coaches Tipp (`FR-T-*`)

- **FR-T-01** — Der Coach gibt vor dem Erzeugen der Monatsrechnungen den Tipp für diesen Monat ein.
- **FR-T-02** — Der Tipp wird je Monat gespeichert; bei erneuter Erzeugung im selben Monat wird der bestehende Tipp vorbefüllt.
- **FR-T-03** — Beim Start des Erzeugen-Workflows fragt die App proaktiv nach dem Tipp, falls noch nicht gesetzt.
- **FR-T-04** — Jeder Athlet kann einen eigenen monatlichen Tipp bekommen (Override des Monats-Default aus FR-T-01). Im Erzeugen-Workflow lässt sich der Tipp pro Athlet individuell anpassen; ohne Override greift der Monats-Default.

### 4.5 Versand / SMTP (`FR-E-*`)

- **FR-E-01** — Optionaler Versand der erzeugten Rechnungen per SMTP, direkt aus der Rechnungsliste.
- **FR-E-02** — SMTP-Zugangsdaten werden verschlüsselt gespeichert (siehe NFR-S-02).
- **FR-E-03** — Der Versand-Button ist aktiv, sobald `SMTP.Enabled = true` ist und der Athlet eine E-Mail-Adresse hinterlegt hat. Andernfalls ist er verborgen.
- **FR-E-04** — Erfolgreicher SMTP-Versand setzt den Status der Rechnung automatisch auf **Versendet**.
- **FR-E-05** — Versand ist nur für Rechnungen im Status **Erstellt** möglich. Versendet- oder Bezahlt-Rechnungen können nicht erneut versendet werden — der Coach muss sie zuerst manuell auf „Erstellt" zurücksetzen.
- **FR-E-06** — Verschlüsselungsmodus der SMTP-Verbindung ist explizit wählbar: „Automatisch" (Port-basiert), „SSL / TLS" (implizites TLS, üblicherweise Port 465), „STARTTLS" (üblicherweise Port 587) oder „Unverschlüsselt" (nur lokale Test-Relays). „Automatisch" entspricht dem v1-Verhalten und ist der Default.

### 4.6 Einstellungen / Stammdaten (`FR-S-*`)

- **FR-S-01** — Coach-Stammdaten: Vorname, Nachname, Adresse, Bank, Kontoinhaber, IBAN, BIC, Steuernummer, UID.
- **FR-S-02** — Finanzamt-Daten: Name (Default „Finanzamt Innsbruck"), Adresse.
- **FR-S-03** — Master-Passwort ändern.
- **FR-S-04** — SMTP-Felder (Host, Port, Verschlüsselung, User, Passwort, Absender, Enabled-Schalter), siehe FR-E-*.
- **FR-S-05** — Daten-Export/Import: Der gesamte Datenbestand kann als unverschlüsselte, eingerückte JSON-Datei exportiert werden (Dateiname `coachly-export-JJJJ-MM-TT.json`). Eine zuvor exportierte JSON-Datei kann wieder importiert werden; der Import ersetzt den gesamten bestehenden Datenbestand und erfordert eine explizite Bestätigung („Ja, alles ersetzen"). Schema-Migration läuft beim Import automatisch.
- **FR-S-06** — Konfigurierbares Rechnungsnummern-Format: Profil aus Tokens (`{YYYY}`, `{YY}`, `{MM}`, `{NNN}`, `{NNNN}`, `{N}`, `{firstname}`, `{lastname}`, `{initials}`). Der Default `{YYYY}{NNN}` entspricht dem v1-Verhalten. Bestehende Rechnungen behalten ihre Anzeige-Nummer (Einfrieren bei Erstellung); nur neue Rechnungen nutzen das aktuelle Format. Die fortlaufende Zählung (`Counter.NextInvoiceNumber`) bleibt monoton und lückenlos (FR-I-03).

### 4.7 Auswertungen (`FR-R-*`) — could-have, **in v1 enthalten**

- **FR-R-01** — Kachel: Umsatz aktueller Monat.
- **FR-R-02** — Kachel: Gesamtumsatz seit Beginn.
- **FR-R-03** — Kachel: Jahresumsatz (aktuelles Kalenderjahr).
- **FR-R-04** — Kachel: Year-to-Date.

---

## 5. Nicht-funktionale Anforderungen

### 5.1 Sicherheit & Verschlüsselung (`NFR-S-*`)

- **NFR-S-01** — Der gesamte Datenstore ist verschlüsselt; auf der Festplatte liegt **kein Klartext** sensibler Daten.
- **NFR-S-02** — Zugang zur App nur über Master-Passwort (Argon2id-KDF → AES-256-GCM).
- **NFR-S-03** — Master-Passwort wird **nie** gespeichert. Verlust = Datenverlust (im UI klar kommunizieren).
- **NFR-S-04** — Atomare Schreibvorgänge (`tmp + fsync + rename`), damit ein Crash mitten im Speichern den Store nicht beschädigt.
- **NFR-S-05** — SMTP-Zugangsdaten, IBAN, USt-ID, Steuernummer, Athleten-Adressen sind Teil des verschlüsselten Stores.

### 5.2 Distribution & Laufzeit (`NFR-D-*`)

- **NFR-D-01** — Single-Binary-Distribution; Doppelklick startet die App.
- **NFR-D-02** — Mac: universelles Binary (arm64 + amd64) in `coachly.app`-Bundle, gepackt als `coachly-mac.zip`.
- **NFR-D-03** — Windows: `coachly.exe` mit `-H windowsgui` (keine Konsole), gepackt als `coachly-win.zip`.
- **NFR-D-04** — Keine Laufzeit-Abhängigkeiten (kein Node, Python, Chrome, JRE …) auf dem Zielsystem.
- **NFR-D-05** — Daten- und PDF-Verzeichnis: `~/Library/Application Support/coachly/` (Mac) bzw. `%APPDATA%\coachly\` (Windows).

### 5.3 Sprache & Lokalisierung (`NFR-L-*`)

- **NFR-L-01** — UI vollständig auf Deutsch.
- **NFR-L-02** — Alle UI-Strings laufen über eine zentrale i18n-Map; keine hartcodierten Strings in Handlern/Templates.
- **NFR-L-03** — Datumsformat `TT.MM.JJJJ`, Geldformat `1.234,56 €`.

---

## 6. UI-Referenz

- Referenzbild: [`ui-standard.png`](./ui-standard.png).
- Vereinbarte Vereinfachungen gegenüber dem Mockup:
  - Linke Sidebar nur mit: Dashboard / Athleten / Rechnungen / Auswertungen / Einstellungen / Abmelden.
  - **Keine** Plans & Packages, Expenses, Calendar, Messages.
- Designprinzipien: dunkle Sidebar (`slate-900`), helles Card-Layout, Teal-Akzent (`teal-500`), durchgehend abgerundete Ecken (`rounded-2xl` für Karten, `rounded-lg` für Inputs, `rounded-full` für primäre CTAs).
- Donut-Chart für Rechnungsstatus (bezahlt / offen) via Chart.js.

---

## 7. Out of Scope (v1)

- Kalender, Sessions, Messaging-Features aus dem Mockup.
- Buchhaltungs-Export, DATEV-Schnittstelle.
- Mehrsprachigkeit (English-Toggle).
- Mehrere Coaches pro Instanz.

---

## 8. Offene Fragen

| ID | Frage | Status |
|---|---|---|
| Q-01 | Final-App-Icon (1024×1024 PNG)? | Platzhalter in v1 |
| Q-02 | Demo-Datensatz beim ersten Start anbieten? | Vermutlich ja, hinter Button auf leerem Dashboard |
| Q-03 | Apple Developer Cert für Signing? | Nein in v1; Right-Click → Öffnen Workaround dokumentiert |

---

## 9. Änderungsprotokoll

| Datum | IDs | Änderung |
|---|---|---|
| 2026-05-17 | initial | Erstfassung, restrukturiert aus dem ursprünglichen Freitext. |
| 2026-05-17 | FR-P-09 | Rechnungsnummer dem PDF-Dateinamen hinzugefügt, um Kollisionen bei gleichen Vornamen zu vermeiden. |
| 2026-05-17 | FR-T-04 | Per-Athlet-Tipp-Override pro Monat ergänzt. |
| 2026-05-17 | FR-P-10 | Konfigurierbarer Speicherort für Rechnungs-PDFs in den Einstellungen ergänzt; Standard bleibt das App-Datenverzeichnis. |
| 2026-05-17 | FR-A-06, FR-S-05 | Plaintext-JSON-Export/-Import des gesamten Datenbestands sowie Athleten-Bulk-Import per CSV ergänzt (Vorbereitung für Versionskontrolle). |
| 2026-05-17 | FR-I-07, FR-I-08, FR-I-09 | Rechnungs-Lifecycle um „Versendet" erweitert; Schutz von Versendet/Bezahlt vor Löschung/Überschreibung; Einzelrechnung-Flow; dritte Konflikt-Option „mit selber Nummer neu erstellen". |
| 2026-05-17 | FR-E-01..05, FR-S-04 | SMTP-Versand aus v2 in v1 promoted: Versand per Klick auf erstellte Rechnungen, SMTP-Enabled-Schalter, automatischer Statuswechsel auf Versendet bei Erfolg. |
| 2026-05-17 | FR-E-06 | Verschlüsselungsmodus der SMTP-Verbindung explizit wählbar (Auto / SSL / STARTTLS / Unverschlüsselt), damit Provider mit blockiertem STARTTLS-Port via SSL auf 465 nutzbar sind. |
| 2026-05-17 | FR-S-06 | Konfigurierbares Rechnungsnummern-Format mit Token-Profil; Anzeige-Nummer bei Erstellung eingefroren. |
