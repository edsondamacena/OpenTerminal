"""Gera as imagens do instalador NSIS a partir de build/appicon.png.

- build/windows/installer/resources/welcome.bmp (164x314): painel das telas de boas-vindas e
  conclusão, grafite com o logo e o nome.
- build/windows/installer/resources/header.bmp (150x57): logo no cabeçalho das páginas internas.

Rodar da raiz do repositório: python scripts/installer-art.py
"""
from pathlib import Path

from PIL import Image, ImageDraw, ImageFont

ROOT = Path(__file__).resolve().parent.parent
OUT = ROOT / 'build' / 'windows' / 'installer' / 'resources'
FONTS = Path('C:/Windows/Fonts')

TOP = (34, 36, 43)
BOTTOM = (16, 17, 20)
WHITE = (255, 255, 255)
MUTED = (160, 164, 176)
ACCENT = (0, 137, 253)  # azul do appicon do OpenTerminal (o laranja era do OpenHost)

SCALE = 3  # desenha em 3x e reduz, para o texto sair liso


# Fora do Windows (build no Linux): Noto Sans no lugar da Segoe UI.
LINUX_FONTS = {'segoeuib.ttf': '/usr/share/fonts/truetype/noto/NotoSans-Bold.ttf',
               'segoeui.ttf': '/usr/share/fonts/truetype/noto/NotoSans-Regular.ttf'}


def font(name: str, size: int) -> ImageFont.FreeTypeFont:
    path = FONTS / name
    if not path.is_file() and name in LINUX_FONTS:
        path = Path(LINUX_FONTS[name])
    return ImageFont.truetype(str(path), size * SCALE)


def gradient(w: int, h: int) -> Image.Image:
    img = Image.new('RGB', (w, h))
    px = img.load()
    for y in range(h):
        t = y / max(h - 1, 1)
        row = tuple(round(TOP[i] + (BOTTOM[i] - TOP[i]) * t) for i in range(3))
        for x in range(w):
            px[x, y] = row
    return img


def centered(draw: ImageDraw.ImageDraw, y: int, text: str, f, fill, width: int) -> None:
    left, _, right, _ = draw.textbbox((0, 0), text, font=f)
    draw.text(((width - (right - left)) / 2 - left, y), text, font=f, fill=fill)


def welcome(icon: Image.Image) -> Image.Image:
    w, h = 164 * SCALE, 314 * SCALE
    img = gradient(w, h)
    size = 92 * SCALE
    img.paste(icon.resize((size, size), Image.LANCZOS), ((w - size) // 2, 78 * SCALE), icon.resize((size, size), Image.LANCZOS))
    draw = ImageDraw.Draw(img)
    centered(draw, 184 * SCALE, 'OpenTerminal', font('segoeuib.ttf', 19), WHITE, w)
    centered(draw, 212 * SCALE, 'Cliente SSH para Windows', font('segoeui.ttf', 9), MUTED, w)
    bar_w = 26 * SCALE
    draw.rounded_rectangle(
        [(w - bar_w) // 2, 236 * SCALE, (w + bar_w) // 2, 238 * SCALE], radius=SCALE, fill=ACCENT
    )
    return img.resize((164, 314), Image.LANCZOS)


def header(icon: Image.Image) -> Image.Image:
    # O MUI desenha o cabeçalho sobre fundo branco: logo à direita, resto branco.
    w, h = 150, 57
    img = Image.new('RGB', (w * SCALE, h * SCALE), WHITE)
    size = 42 * SCALE
    small = icon.resize((size, size), Image.LANCZOS)
    img.paste(small, (w * SCALE - size - 8 * SCALE, (h * SCALE - size) // 2), small)
    return img.resize((w, h), Image.LANCZOS)


def main() -> None:
    icon = Image.open(ROOT / 'build' / 'appicon.png').convert('RGBA')
    OUT.mkdir(parents=True, exist_ok=True)
    welcome(icon).save(OUT / 'welcome.bmp')
    header(icon).save(OUT / 'header.bmp')
    print(f'imagens em {OUT}')


if __name__ == '__main__':
    main()
