# Changelog techniczny

Zmiany, których użytkownik narzędzia nie zobaczy: układ warstw, strażnicy,
konfiguracja CI, decyzje architektoniczne. Zmiany widoczne dla użytkownika
idą do `CHANGELOG.md` i są po angielsku.

## Nieopublikowane

### 2026-08-01 — szkielet repozytorium

- Układ katalogów i granice warstw wg `ARCHITECTURE.md §2`. Pakiety puste,
  z samymi komentarzami pakietowymi.
- **Czterej strażnicy uzbrojeni przed pierwszym kodem funkcjonalnym**, każdy
  sprawdzony mutacją:
  - test warstw — nic nie wskazuje w górę, `cmd/tfg` nie tknie GUI
  - test izolacji sieciowej — zakaz `net` i `os/exec` w dolnych warstwach
  - skan ASCII — CLI wyłącznie po angielsku
  - test stabilności bajtowej — sześć ścieżek biblioteki standardowej
- `LICENSE` — kanoniczny tekst GPL-3.0, SHA-256 `8ceb4b9e…`, skopiowany
  z lokalnej dystrybucji zamiast odtwarzany.
- `.gitignore` sprawdzony pomiarem na 21 ścieżkach. Wyłapał własny błąd:
  negacja `testdata/` zakotwiczona w korzeniu gubiła `testdata/` per pakiet,
  czyli konwencję Go.
- `.gitattributes` — końce linii normalizowane do LF na każdym systemie,
  `testdata/` i `LICENSE` wyłączone z jakiejkolwiek konwersji. Bez tego git
  na Windows przepisywałby wzorce porównywane bajt w bajt, a awaria
  wyglądałaby jak dryf kompresora, nie jak git dotykający pliku.
- Wartości wzorcowe stabilności bajtowej **zmierzone od nowa** na go1.26.5.
  Nie zgadzają się z prefiksami z `STACK.md §4.1`, bo tamten pomiar używał
  innego wsadu — dzisiejszy test definiuje swój wsad w sobie.
