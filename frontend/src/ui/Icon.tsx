// Único ponto de entrada de ícones (FR-169): Lucide em traço, tamanhos dos tokens.
import type { LucideIcon, LucideProps } from 'lucide-react';

export type IconSize = 'sm' | 'md' | 'lg';

const PX: Record<IconSize, number> = { sm: 16, md: 20, lg: 24 };
const STROKE = 1.75;

export type IconProps = Omit<LucideProps, 'size'> & { size?: IconSize };

/** Envolve um ícone do Lucide com o traço e os tamanhos padrão; decorativo sem `aria-label`. */
export function icon(Glyph: LucideIcon) {
  function Icon({ size = 'sm', width, height, ...props }: IconProps) {
    const px = PX[size];
    const labelled = props['aria-label'] !== undefined;
    return (
      <Glyph
        width={width ?? px}
        height={height ?? px}
        // Traço constante em px em qualquer tamanho (o viewBox do Lucide é 24).
        strokeWidth={(STROKE * 24) / Number(width ?? px)}
        aria-hidden={labelled ? undefined : true}
        focusable="false"
        role={labelled ? 'img' : undefined}
        {...props}
      />
    );
  }
  Icon.displayName = `Icon(${Glyph.displayName ?? 'lucide'})`;
  return Icon;
}
