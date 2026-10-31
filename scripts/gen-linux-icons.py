#!/usr/bin/env python3
"""Ícones do Linux e do favicon a partir de build/appicon.png (1024 px), com margem de ícone de app.

31/10: os ícones pequenos (32/48/64) saíam sem margem, encostando na borda ("parece com zoom") e
borrados. Aqui o desenho ocupa MIOLO do quadro, é reduzido em etapas (sem serrilhado) e os tamanhos
pequenos ganham uma nitidez leve. Saída: build/linux/icons/<s>.png e frontend/public/favicon*.png.
"""
import sys
from pathlib import Path
from PIL import Image, ImageFilter

ROOT = Path(__file__).resolve().parent.parent
MIOLO = 0.86  # fração do quadro ocupada pelo desenho (o resto é margem transparente)
TAMANHOS = (16, 24, 32, 48, 64, 128, 256, 512)


def icone(src, s):
    arte = src.crop(src.split()[3].getbbox())  # só o desenho, sem a margem original
    lado = max(1, round(s * MIOLO))
    w, h = arte.size
    esc = lado / max(w, h)
    alvo = (max(1, round(w * esc)), max(1, round(h * esc)))
    while arte.width > alvo[0] * 2:  # reduz pela metade até perto do alvo: menos serrilhado
        arte = arte.resize((arte.width // 2, arte.height // 2), Image.LANCZOS)
    arte = arte.resize(alvo, Image.LANCZOS)
    if s <= 64:
        arte = arte.filter(ImageFilter.UnsharpMask(radius=0.6, percent=60, threshold=1))
    quadro = Image.new("RGBA", (s, s), (0, 0, 0, 0))
    quadro.paste(arte, ((s - alvo[0]) // 2, (s - alvo[1]) // 2), arte)
    return quadro


def main():
    src = Image.open(ROOT / "build" / "appicon.png").convert("RGBA")
    out = ROOT / "build" / "linux" / "icons"
    out.mkdir(parents=True, exist_ok=True)
    for s in TAMANHOS:
        icone(src, s).save(out / f"{s}.png", optimize=True)
    pub = ROOT / "frontend" / "public"
    icone(src, 256).save(pub / "favicon.png", optimize=True)
    icone(src, 32).save(pub / "favicon-32.png", optimize=True)
    print(f"ícones em {out} e {pub}", file=sys.stderr)


if __name__ == "__main__":
    main()
