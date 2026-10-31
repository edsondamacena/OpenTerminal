#!/usr/bin/env bash
# Gera o OpenTerminal para Android arm64.
#   sem argumento: APK de teste (debug)
#   release: AAB para a Play Store + APK assinado, com a chave de upload
#            (~/.config/OpenTerminal/release/signing.properties, fora do git).
# Requer: Go, Node, JDK 17 (~/.local/jdk-17), Android SDK/NDK (~/Android/Sdk), Gradle (~/.local/gradle).
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/.." && pwd)
export JAVA_HOME=${JAVA_HOME:-$HOME/.local/jdk-17}
export ANDROID_HOME=${ANDROID_HOME:-$HOME/Android/Sdk}
export PATH=$HOME/.local/go/bin:$JAVA_HOME/bin:$PATH
GRADLE=${GRADLE:-$HOME/.local/gradle/bin/gradle}
MODE=${1:-debug}

echo "== interface"
(cd "$ROOT/frontend" && npm run build >/dev/null)

echo "== núcleo Go (android/arm64)"
OUT="$ROOT/android/app/src/main/jniLibs/arm64-v8a/libotcore.so"
OAUTH=$("$ROOT/scripts/oauth-ldflags.sh")
NDK=$(ls -d "$ANDROID_HOME"/ndk/* 2>/dev/null | sort -V | tail -1 || true)
CC="$NDK/toolchains/llvm/prebuilt/linux-x86_64/bin/aarch64-linux-android26-clang"
if [ -x "$CC" ]; then
  # Com cgo o Go usa o resolvedor DNS do Android; sem cgo ele procura /etc/resolv.conf, que não existe.
  CGO_ENABLED=1 CC="$CC" GOOS=android GOARCH=arm64 go build -C "$ROOT" -trimpath -ldflags "-s -w $OAUTH" -o "$OUT" ./cmd/openterminal-android
else
  echo "aviso: NDK não encontrado; núcleo sem cgo (nomes de host podem não resolver)" >&2
  CGO_ENABLED=0 GOOS=android GOARCH=arm64 go build -C "$ROOT" -trimpath -ldflags "-s -w $OAUTH" -o "$OUT" ./cmd/openterminal-android
fi

echo "== APK"
echo "sdk.dir=$ANDROID_HOME" > "$ROOT/android/local.properties"
VERSION=$(node -p "require('$ROOT/package.json').version")
mkdir -p "$ROOT/build/bin"
if [ "$MODE" = release ]; then
  "$GRADLE" -q -p "$ROOT/android" bundleRelease assembleRelease
  cp "$ROOT/android/app/build/outputs/bundle/release/app-release.aab" "$ROOT/build/bin/OpenTerminal-$VERSION.aab"
  mkdir -p "$ROOT/build/bin/Instaladores"
  cp "$ROOT/android/app/build/outputs/apk/release/app-release.apk" "$ROOT/build/bin/Instaladores/OpenTerminal-$VERSION.apk"
  ls -la "$ROOT/build/bin/OpenTerminal-$VERSION.aab" "$ROOT/build/bin/Instaladores/OpenTerminal-$VERSION.apk"
else
  "$GRADLE" -q -p "$ROOT/android" assembleDebug
  cp "$ROOT/android/app/build/outputs/apk/debug/app-debug.apk" "$ROOT/build/bin/OpenTerminal-$VERSION-debug.apk"
  ls -la "$ROOT/build/bin/OpenTerminal-$VERSION-debug.apk"
fi
