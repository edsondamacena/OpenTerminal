#!/usr/bin/env bash
# Gera o binário Linux (build/bin/openterminal), com o cliente OAuth do Google embutido.
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/.." && pwd)
export PATH=$HOME/.local/go/bin:$HOME/go/bin:$PATH
OAUTH=$("$ROOT/scripts/oauth-ldflags.sh")
cd "$ROOT"
wails build -tags webkit2_41 -trimpath -ldflags "-s -w $OAUTH" -o openterminal.new
mv build/bin/openterminal.new build/bin/openterminal
ls -la build/bin/openterminal
