// Cor de destaque (FR-171): uma das opções da marca ou a do Windows, sempre com contraste
// mínimo de 4,5:1 contra as superfícies do modo.
import { BRAND, BRAND_ACCENTS, type BrandAccentId } from './brand.gen';
import { contrast, parseColor } from './color';

export type AccentId = BrandAccentId | 'system';

export const DEFAULT_ACCENT: BrandAccentId = 'laranja';

export interface AccentTokens {
  accent: string;
  accentGradient: string;
  accentSoft: string;
  accentText: string;
  focus: string;
  selected: string;
}

const MIN = 4.5;
/** Superfícies onde o destaque aparece como texto ou anel: moldura e diálogos de cada tema. */
const SURFACES = {
  dark: [BRAND.laca, BRAND.superficie],
  light: [BRAND.papel, BRAND.branco],
};

type Oklch = [number, number, number];

function srgbToLinear(c: number): number {
  return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
}

function linearToSrgb(c: number): number {
  return c <= 0.0031308 ? 12.92 * c : 1.055 * c ** (1 / 2.4) - 0.055;
}

export function toOklch(hex: string): Oklch {
  const { r, g, b } = parseColor(hex);
  const [lr, lg, lb] = [r, g, b].map((v) => srgbToLinear(v / 255)) as [number, number, number];
  const l = Math.cbrt(0.4122214708 * lr + 0.5363325363 * lg + 0.0514459929 * lb);
  const m = Math.cbrt(0.2119034982 * lr + 0.6806995451 * lg + 0.1073969566 * lb);
  const s = Math.cbrt(0.0883024619 * lr + 0.2817188376 * lg + 0.6299787005 * lb);
  const L = 0.2104542553 * l + 0.793617785 * m - 0.0040720468 * s;
  const A = 1.9779984951 * l - 2.428592205 * m + 0.4505937099 * s;
  const B = 0.0259040371 * l + 0.7827717662 * m - 0.808675766 * s;
  const C = Math.hypot(A, B);
  const h = ((Math.atan2(B, A) * 180) / Math.PI + 360) % 360;
  return [L, C, h];
}

function oklchToRgb([L, C, h]: Oklch): [number, number, number] {
  const a = C * Math.cos((h * Math.PI) / 180);
  const b = C * Math.sin((h * Math.PI) / 180);
  const l = (L + 0.3963377774 * a + 0.2158037573 * b) ** 3;
  const m = (L - 0.1055613458 * a - 0.0638541728 * b) ** 3;
  const s = (L - 0.0894841775 * a - 1.291485548 * b) ** 3;
  return [
    4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s,
    -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s,
    -0.0041960863 * l - 0.7034186147 * m + 1.707614701 * s,
  ];
}

/** Converte de volta reduzindo o croma até caber no sRGB, sem mudar luminosidade e matiz. */
export function fromOklch([L, C, h]: Oklch): string {
  let chroma = C;
  let rgb = oklchToRgb([L, chroma, h]);
  while (chroma > 0 && rgb.some((v) => v < -0.0005 || v > 1.0005)) {
    chroma = Math.max(0, chroma - 0.005);
    rgb = oklchToRgb([L, chroma, h]);
  }
  const hex = rgb
    .map((v) => Math.round(Math.min(1, Math.max(0, linearToSrgb(v))) * 255))
    .map((v) => v.toString(16).padStart(2, '0'))
    .join('');
  return `#${hex}`;
}

function minContrast(color: string, dark: boolean): number {
  const c = parseColor(color);
  return Math.min(...SURFACES[dark ? 'dark' : 'light'].map((s) => contrast(c, parseColor(s))));
}

/** Clareia (escuro) ou escurece (claro) em OKLCH até a cor passar de 4,5:1 nas superfícies. */
export function ensureContrast(hex: string, dark: boolean): string {
  const [L0, C, h] = toOklch(hex);
  let color = fromOklch([L0, C, h]);
  for (let step = 1; minContrast(color, dark) < MIN && step <= 100; step++) {
    const L = Math.min(1, Math.max(0, L0 + (dark ? 0.01 : -0.01) * step));
    color = fromOklch([L, C, h]);
  }
  return color;
}

function alpha(hex: string, a: number): string {
  const { r, g, b } = parseColor(hex);
  return `rgba(${String(r)}, ${String(g)}, ${String(b)}, ${String(a)})`;
}

/** Texto sobre o destaque (botão primário): o que contrastar mais entre laca e branco. */
function textOn(accent: string): string {
  const bg = parseColor(accent);
  return contrast(parseColor(BRAND.laca), bg) >= contrast(parseColor(BRAND.branco), bg)
    ? BRAND.laca
    : BRAND.branco;
}

export function resolveAccent(id: string, dark: boolean, systemAccent: string | null): string {
  if (id === 'system' && systemAccent) return ensureContrast(systemAccent, dark);
  const brand =
    id in BRAND_ACCENTS ? BRAND_ACCENTS[id as BrandAccentId] : BRAND_ACCENTS[DEFAULT_ACCENT];
  return dark ? brand.dark : brand.light;
}

export function accentTokens(id: string, dark: boolean, systemAccent: string | null): AccentTokens {
  const accent = resolveAccent(id, dark, systemAccent);
  const [L, C, h] = toOklch(accent);
  const top = fromOklch([Math.min(1, L + (dark ? 0.06 : 0.05)), C, h]);
  return {
    accent,
    accentGradient: `linear-gradient(180deg, ${top} 0%, ${accent} 100%)`,
    accentSoft: alpha(accent, dark ? 0.3 : 0.24),
    accentText: textOn(accent),
    focus: accent,
    selected: alpha(accent, dark ? 0.16 : 0.12),
  };
}
