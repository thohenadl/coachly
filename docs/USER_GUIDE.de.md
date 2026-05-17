# coachly — Benutzerhandbuch

Willkommen bei coachly! Dieses Handbuch richtet sich an dich als Coach. Du brauchst keinerlei Programmierkenntnisse — alles funktioniert per Doppelklick.

---

## 1. Installation

### Mac

1. Lade `coachly-mac.zip` herunter und entpacke die Datei (Doppelklick).
2. Verschiebe `coachly.app` in deinen `Programme`-Ordner.
3. **Erster Start:** macOS blockiert unsignierte Apps. **Rechtsklick** auf `coachly.app` → **Öffnen** → **Öffnen** im Dialog bestätigen. Nur beim ersten Mal nötig.
4. Danach reicht ein normaler Doppelklick auf das App-Icon im Dock oder im Programme-Ordner.

> Warum die Rechtsklick-Prozedur? coachly ist nicht bei Apple signiert (das kostet 99 USD/Jahr). Apple zeigt deshalb eine Warnung. Der Rechtsklick-Trick ist Apples eingebauter Weg, der App einmalig zu vertrauen.

### Windows

1. Lade `coachly-win.zip` herunter und entpacke es.
2. **Doppelklick** auf `coachly.exe`.
3. Beim ersten Start zeigt Windows eventuell eine SmartScreen-Warnung („Computer wurde geschützt"). Klicke **„Weitere Informationen"** → **„Trotzdem ausführen"**.

---

## 2. Erststart: Master-Passwort

Beim allerersten Start fragt coachly nach einem **Master-Passwort**. Damit werden alle deine Daten (Athleten, Rechnungen, Bankverbindung …) verschlüsselt.

**Wichtig:**
- Das Passwort wird **nicht** gespeichert — coachly kennt es nicht.
- **Verlust = Datenverlust.** Es gibt keine Wiederherstellung.
- Wähle ein Passwort mit mindestens 8 Zeichen. Empfehlung: ein langer, einprägsamer Satz.

Bei jedem späteren Start fragt coachly nur noch nach diesem Passwort, um die Daten zu entsperren.

---

## 3. Erste Schritte (Checkliste)

Nach dem Erststart und dem Anlegen des Passworts:

1. **Stammdaten ausfüllen** (`Einstellungen → Coach-Stammdaten`)
   - Vorname, Nachname, Adresse
   - Bank, Kontoinhaber, IBAN, BIC
   - Steuernummer, UID (USt-ID)
2. **Finanzamt** (`Einstellungen → Finanzamt`)
   - Default: „Finanzamt Innsbruck". Anpassen, falls anderes Finanzamt zuständig.
3. **Athleten anlegen** (`Athleten → + Athlet hinzufügen`)
   - Pro Athlet: Name, Adresse, monatliche Gebühr, Coaching-Start.
   - Bei Start mitten im Monat berechnet coachly die erste Rechnung automatisch anteilig.
4. **Erste Rechnungen erzeugen** (`Rechnungen → + Rechnungen erzeugen`)
   - Monat auswählen.
   - Coaches Tipp für diesen Monat eintragen (Default für alle Athleten).
   - Optional: pro Athlet einen abweichenden Tipp eintragen.
   - **Vorschau** durchklicken — links/rechts navigieren.
   - **Alle bestätigen & PDF erzeugen** klickt den Workflow ab.

---

## 4. Wo werden meine Daten gespeichert?

- **Mac:** `~/Library/Application Support/coachly/`
- **Windows:** `%APPDATA%\coachly\` (üblicherweise `C:\Users\<Du>\AppData\Roaming\coachly\`)

In diesem Ordner findest du:

| Datei / Ordner | Inhalt |
|---|---|
| `store.enc` | Verschlüsselte Datenbank (Athleten, Rechnungen, Stammdaten). Ohne dein Passwort unlesbar. |
| `invoices/` | Erzeugte Rechnungs-PDFs. Dateinamen: `vorname_monat_jahr_rechnungsnummer.pdf`. |

---

## 5. Backup

coachly hat (noch) keine eingebaute Backup-Funktion — aber das Backup ist einfach:

> **Kopiere den gesamten coachly-Ordner regelmäßig** (siehe Pfad oben) auf eine externe Festplatte, einen USB-Stick oder in deinen iCloud-/Dropbox-Ordner.

`store.enc` ist verschlüsselt, du kannst die Datei also bedenkenlos in der Cloud sichern.

**Wiederherstellung:** Den Ordner einfach an dieselbe Stelle zurückkopieren und coachly starten.

---

## 6. Passwort ändern

`Einstellungen → Passwort`. Aktuelles Passwort + neues Passwort zweimal eingeben. Die Datendatei wird automatisch mit dem neuen Passwort neu verschlüsselt.

---

## 7. Häufige Probleme

| Problem | Lösung |
|---|---|
| Mac: „coachly kann nicht geöffnet werden, da der Entwickler nicht verifiziert werden kann." | Rechtsklick auf die App → **Öffnen** → im Dialog erneut **Öffnen**. Nur beim ersten Start nötig. |
| Browser öffnet sich nicht automatisch | Schau in der Konsole / dem Mini-Fenster nach einer Adresse wie `http://127.0.0.1:51234/` und öffne sie manuell. |
| Falsches Passwort eingegeben | Einfach nochmal versuchen. Es gibt keine Sperre, aber jeder Entsperr-Versuch dauert ~300 ms (Argon2id). |
| Ich habe mein Passwort vergessen | Es gibt keine Wiederherstellung — die Daten sind verloren. `store.enc` löschen und neu beginnen. |

---

## 8. App komplett zurücksetzen

Manchmal möchtest du coachly auf den Werkszustand zurücksetzen — z. B. weil du das Passwort vergessen hast, mit echten Daten neu starten willst oder die App an jemand anderen weitergibst.

> ⚠️ **Achtung:** Das löscht **alle** Athleten, Rechnungen, Stammdaten und PDFs unwiderruflich. Mache vorher ein Backup (siehe §5), wenn du die Daten noch brauchst.

### Mac

1. **Finder** öffnen.
2. Im Menü **Gehe zu → Gehe zum Ordner …** (oder `⇧⌘G`).
3. Pfad eingeben: `~/Library/Application Support/coachly` und Enter.
4. Den gesamten Ordner-Inhalt in den Papierkorb verschieben (oder den ganzen `coachly`-Ordner löschen).
5. coachly neu starten — du landest wieder beim Erststart-Passwortdialog.

Alternative im Terminal:

```bash
rm -rf ~/Library/Application\ Support/coachly
```

### Windows

1. **Datei-Explorer** öffnen.
2. In die Adressleiste `%APPDATA%\coachly` eingeben und Enter.
3. Den gesamten Ordner-Inhalt löschen (oder den `coachly`-Ordner darüber komplett).
4. coachly neu starten — du landest wieder beim Erststart-Passwortdialog.

### Nur Teile zurücksetzen

| Was du löschen willst | Datei / Ordner |
|---|---|
| Alles (Passwort, Athleten, Rechnungen, PDFs) | gesamter `coachly`-Ordner |
| Nur die Datenbank (Passwort + Athleten + Rechnungen), PDFs behalten | `store.enc` |
| Nur die erzeugten PDFs, Datenbank behalten | `invoices/` |

---

## 9. Updates

Neue coachly-Version: einfach das neue Zip herunterladen, alte App ersetzen, fertig. Deine Daten (`store.enc`, `invoices/`) bleiben unangetastet, weil sie in einem separaten Ordner liegen.
