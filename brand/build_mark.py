"""Constrói a marca mestra do OpenTerminal (brand/mark.svg) com geometria real, a partir do logo
escolhido (brand/logo-source.png): anel aberto à direita como um "O" e o chevron do prompt.

Grade de 240 unidades (10x a do Logo.tsx, viewBox 24). Rodar da raiz: python brand/build_mark.py
"""
import math
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT / '.claude' / 'skills' / 'brand-kit' / 'scripts'))
from geometry import circle_d, pt, subtract, to_path, union, write_svg  # noqa: E402

C = 120.0          # centro
R = 48.9           # raio do eixo do anel
RING = 11.0        # espessura do anel
GAP = 12.0         # meia abertura do anel, em graus, em volta do eixo horizontal direito
CHEV = 12.0        # espessura do chevron
CHEVRON = [(104.5, 99.0), (126.0, 120.0), (104.5, 141.0)]


def capsule(a, b, w):
    """Segmento a-b com pontas redondas e largura w."""
    (x1, y1), (x2, y2) = a, b
    ang = math.atan2(y2 - y1, x2 - x1)
    nx, ny = -math.sin(ang) * w / 2, math.cos(ang) * w / 2
    body = to_path(
        f'M{x1 + nx},{y1 + ny} L{x2 + nx},{y2 + ny} L{x2 - nx},{y2 - ny} L{x1 - nx},{y1 - ny} Z'
    )
    return union(body, to_path(circle_d(x1, y1, w / 2)), to_path(circle_d(x2, y2, w / 2)))


def ring():
    band = subtract(to_path(circle_d(C, C, R + RING / 2)), to_path(circle_d(C, C, R - RING / 2)))
    # Cunha que abre o anel à direita.
    far = R * 2
    a, b = pt(far, -GAP, C, C), pt(far, GAP, C, C)
    wedge = to_path(f'M{C},{C} L{a[0]},{a[1]} L{far * 2 + C},{C} L{b[0]},{b[1]} Z')
    band = subtract(band, wedge)
    caps = [to_path(circle_d(*pt(R, d, C, C), RING / 2)) for d in (-GAP, GAP)]
    return union(band, *caps)


def chevron():
    return union(capsule(CHEVRON[0], CHEVRON[1], CHEV), capsule(CHEVRON[1], CHEVRON[2], CHEV))


def main():
    out = ROOT / 'brand' / 'mark.svg'
    write_svg(str(out), [(ring(), '#FFFFFF', 'main'), (chevron(), '#FD7B17', 'accent')])
    print(f'marca em {out}')


if __name__ == '__main__':
    main()
