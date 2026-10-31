/** Preenchimento da pastilha de avatar a partir do matiz estável do host (FR-163). */
export function avatarFill(hue: number): string {
  return `linear-gradient(140deg, hsl(${String(hue)} 55% 56%), hsl(${String(hue + 35)} 58% 42%))`;
}
