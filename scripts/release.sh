#!/usr/bin/env bash
# Publica a versão do wails.json como Release no GitHub, com os instaladores de build/bin/Instaladores/
# e o SHA256SUMS (o botão Atualizar recusa instalar sem ele). Rode depois dos builds.
#   scripts/release.sh                 # publica
#   scripts/release.sh --rascunho      # cria como rascunho (não aparece no Atualizar)
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/.." && pwd)
VERSION=$(python3 -I -c 'import json,sys; print(json.load(open(sys.argv[1]))["info"]["productVersion"])' "$ROOT/wails.json")
DIR="$ROOT/build/bin/Instaladores"
cd "$DIR"
ARQS=()
for f in "openterminal_${VERSION}_amd64.deb" OpenTerminal-amd64-installer.exe "OpenTerminal-${VERSION}.apk"; do
  [ -f "$f" ] && ARQS+=("$f") || echo "aviso: $f não encontrado (fica fora da Release)" >&2
done
[ ${#ARQS[@]} -gt 0 ] || { echo "nenhum instalador em $DIR" >&2; exit 1; }
sha256sum "${ARQS[@]}" > SHA256SUMS
cat SHA256SUMS
EXTRA=()
[ "${1:-}" = "--rascunho" ] && EXTRA+=(--draft)
NOTAS="$ROOT/build/bin/notas-v${VERSION}.md"
[ -f "$NOTAS" ] || printf 'OpenTerminal %s\n\nFork do OpenHost, de Emerson (EmersonRicard0). Ver AUTORIZACAO.md.\n' "$VERSION" > "$NOTAS"
gh release create "v${VERSION}" "${ARQS[@]}" SHA256SUMS --title "OpenTerminal ${VERSION}" --notes-file "$NOTAS" "${EXTRA[@]}"
