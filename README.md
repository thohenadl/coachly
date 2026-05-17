# coachly

Schlanke Desktop-App zur Verwaltung von Coaching-Athleten und zur monatlichen Rechnungsstellung. Eine einzige ausführbare Datei pro Betriebssystem — die Daten bleiben lokal und verschlüsselt auf dem eigenen Rechner.

| Zielgruppe | Wegweiser |
|---|---|
| **Du möchtest coachly nutzen** (Coach, kein Entwickler) | → [`docs/USER_GUIDE.de.md`](docs/USER_GUIDE.de.md) |
| **Du möchtest coachly für Mac bauen** | → [`docs/BUILD_MAC.md`](docs/BUILD_MAC.md) |
| **Du möchtest coachly für Windows bauen** | → [`docs/BUILD_WIN.md`](docs/BUILD_WIN.md) |
| **Du möchtest am Code arbeiten** | → [`CLAUDE.md`](CLAUDE.md) und [`.requirements/project_requirements.md`](.requirements/project_requirements.md) |

## Stack auf einen Blick

- **Backend:** Go (pure-Go, kein CGO) — `go-chi`, `maroto/v2` für PDFs, AES-256-GCM für die Verschlüsselung.
- **Frontend:** HTML-Templates aus dem Go-Binary, HTMX + Alpine.js für Interaktivität, Chart.js für die Übersichts-Donut, Tailwind v4 (standalone CLI, kein Node).
- **Speicher:** Eine einzige verschlüsselte JSON-Datei im OS-spezifischen Anwendungs-Datenverzeichnis.

## Quickstart (Entwicklung)

```bash
# Einmalig: Tailwind-CSS bauen
./scripts/tailwind.sh

# App starten — öffnet automatisch den Browser
go run .
```

## Release bauen

```bash
./scripts/build-mac.sh   # → dist/coachly-mac.zip
./scripts/build-win.sh   # → dist/coachly-win.zip
```

## Tests

```bash
go test ./...
```
