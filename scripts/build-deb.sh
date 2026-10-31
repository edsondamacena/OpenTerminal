#!/usr/bin/env bash
# Empacota build/bin/openterminal em openterminal_<versão>_amd64.deb.
# Rode antes scripts/build-linux.sh. Saída em build/bin/Instaladores/.
# Instalar: sudo apt install ./build/bin/Instaladores/openterminal_*_amd64.deb
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/.." && pwd)
VERSION=$(node -p "require('$ROOT/package.json').version")
BIN="$ROOT/build/bin/openterminal"
[ -x "$BIN" ] || { echo "falta $BIN: rode scripts/build-linux.sh" >&2; exit 1; }

PKG=$(mktemp -d)
trap 'rm -rf "$PKG"' EXIT
install -Dm755 "$BIN" "$PKG/usr/bin/openterminal"
# 31/10: ícones com margem e nitidez por tamanho (scripts/gen-linux-icons.py), não mais redução direta
python3 -I "$ROOT/scripts/gen-linux-icons.py"
for s in 16 24 32 48 64 128 256 512; do
  install -Dm644 "$ROOT/build/linux/icons/$s.png" "$PKG/usr/share/icons/hicolor/${s}x${s}/apps/openterminal.png"
done
install -Dm644 /dev/stdin "$PKG/usr/share/applications/openterminal.desktop" <<'EOF'
[Desktop Entry]
Type=Application
Name=OpenTerminal
GenericName=Cliente SSH
Comment=Cliente SSH com cofre criptografado
Exec=/usr/bin/openterminal
Icon=openterminal
Terminal=false
Categories=Network;RemoteAccess;System;TerminalEmulator;
Keywords=ssh;sftp;terminal;servidor;
StartupWMClass=OpenTerminal
StartupNotify=true
EOF

# A interface abre no Chrome/Chromium do sistema; sem ele, cai na janela WebKitGTK.
mkdir -p "$PKG/DEBIAN"
cat > "$PKG/DEBIAN/control" <<EOF
Package: openterminal
Version: $VERSION
Section: net
Priority: optional
Architecture: amd64
Maintainer: Edson Damacena <edsondamacenawk@gmail.com>
Depends: libgtk-3-0 | libgtk-3-0t64, libwebkit2gtk-4.1-0
Recommends: google-chrome-stable | chromium | chromium-browser
Installed-Size: $(du -sk "$PKG/usr" | cut -f1)
Description: Cliente SSH local-first com cofre criptografado
 Terminal SSH e local, SFTP, encaminhamento de portas e sync do cofre no Google Drive.
EOF
cat > "$PKG/DEBIAN/postinst" <<'EOF'
#!/bin/sh
set -e
command -v update-desktop-database >/dev/null && update-desktop-database -q /usr/share/applications || true
command -v gtk-update-icon-cache >/dev/null && gtk-update-icon-cache -q -t /usr/share/icons/hicolor || true
EOF
chmod 755 "$PKG/DEBIAN/postinst"

mkdir -p "$ROOT/build/bin/Instaladores"
OUT="$ROOT/build/bin/Instaladores/openterminal_${VERSION}_amd64.deb"
dpkg-deb --root-owner-group --build "$PKG" "$OUT" >/dev/null
ls -la "$OUT"
