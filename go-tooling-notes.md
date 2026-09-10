# Go — notatki z sesji

## .gitignore dla projektu Go

Projekt Go powinien ignorować:

| Kategoria | Wzorce |
|-----------|--------|
| Binaria | `*.exe`, `*.exe~`, `*.dll`, `*.so`, `*.dylib` |
| Testy | `*.test` (binary built with `go test -c`) |
| Coverage/profiling | `*.out`, `*.prof` |
| Workspace | `go.work.local` |
| Vendor (opcjonalnie) | `vendor/` — odkomentować jeśli nie commitujemy vendored deps |

Dodatkowe sekcje w `.gitignore` dla IDE i OS:

- **VSCode:** `.vscode/`
- **IntelliJ/GoLand:** `.idea/`, `*.iml`, `*.iws`, `out/`
- **macOS/Windows:** `.DS_Store`, `Thumbs.db`

---

## govulncheck

### Co to jest

`govulncheck` to oficjalne narzędzie zespołu Go Security do wykrywania znanych podatności (CVE) w zależnościach projektu. Korzysta z bazy [Go Vulnerability Database](https://vuln.go.dev).

Kluczowa przewaga nad prostymi skanerami: analizuje **rzeczywisty call graph** — raportuje tylko te podatności, które kod faktycznie wywołuje (nie wszystkie obecne w drzewie zależności).

### Instalacja

```bash
go install golang.org/x/vuln/cmd/govulncheck@latest
```

Binary ląduje w `$GOPATH/bin` (domyślnie `~/go/bin`, u mnie: `/Users/j.smogorzewski/Documents/go/bin`).

### Wymagana konfiguracja PATH

Jeśli `$GOPATH/bin` nie jest w `$PATH`, dodać do `~/.zshrc`:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

Potem: `source ~/.zshrc`

### Użycie

| Komenda | Opis |
|---------|------|
| `govulncheck ./...` | Skanuje cały projekt (kod + zależności) |
| `govulncheck -mode=binary ./my-app` | Skanuje skompilowany binary |
| `govulncheck -json ./...` | Wynik w JSON (przydatne do CI/CD) |
| `govulncheck -show=verbose ./...` | Szczegółowy raport z pełnym call-chain |

### Co pokazuje wynik

Gdy znajdzie podatność, raportuje:

1. **ID** z bazy (np. `GO-2024-2687`)
2. **Dotknięta zależność** i jej wersja
3. **Wersja z poprawką** (do której zaktualizować)
4. **Czy kod wywołuje podatną funkcję** — rozróżnia "masz tę zależność" od "faktycznie używasz podatnego kodu"

### Typowy workflow

```bash
# W katalogu z go.mod:
govulncheck ./...

# Jeśli coś znajdzie — aktualizacja zależności:
go get example.com/vulnerable-pkg@v1.2.3  # wersja z poprawką
go mod tidy
govulncheck ./...  # ponowna weryfikacja
```
