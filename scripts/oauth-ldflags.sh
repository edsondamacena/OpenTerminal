#!/usr/bin/env bash
# Imprime os -X do cliente OAuth do Google para o go build, lendo o JSON baixado do Google Cloud
# (~/.config/OpenTerminal/google-oauth.json, fora do git). Sem o arquivo, imprime nada e o app
# sai sem o botão do Google.
set -euo pipefail
F=${OPENTERMINAL_OAUTH_JSON:-$HOME/.config/OpenTerminal/google-oauth.json}
[ -r "$F" ] || { echo "aviso: $F não encontrado; build sem login Google" >&2; exit 0; }
python3 -I - "$F" <<'PY'
import json, sys
c = json.load(open(sys.argv[1]))["installed"]
print(f"-X openterminal/internal/gdrive.ClientID={c['client_id']} "
      f"-X openterminal/internal/gdrive.ClientSecret={c['client_secret']}")
PY
