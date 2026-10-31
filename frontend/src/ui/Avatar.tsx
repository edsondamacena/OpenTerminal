import type { OsId } from '@shared/model/types';
import { avatarFill } from '../theme/avatar';
import { OsLogo } from './osIcons';
import s from './ui.module.css';

/** Matiz estável derivado do texto, para cada host ter sempre a mesma cor. */
export function hueOf(text: string): number {
  let h = 0;
  for (const ch of text) h = (h * 31 + (ch.codePointAt(0) ?? 0)) % 360;
  return h;
}

export function initialsOf(text: string): string {
  const words = text
    .split(/[\s._-]+/)
    .map((w) => Array.from(w)[0] ?? '')
    .filter((c) => c !== '');
  return (words.length > 1 ? (words[0] ?? '') + (words[1] ?? '') : (words[0] ?? '?')).toUpperCase();
}

export interface AvatarProps {
  text: string;
  size?: number;
  /** Logo do sistema no lugar das iniciais (FR-178). */
  os?: OsId | null;
}

export function Avatar({ text, size = 28, os = null }: AvatarProps) {
  return (
    <span
      className={s.avatar}
      aria-hidden="true"
      style={
        {
          '--avatar-fill': avatarFill(hueOf(text)),
          width: size,
          height: size,
        } as React.CSSProperties
      }
    >
      {os ? <OsLogo id={os} size={Math.round(size * 0.57)} /> : initialsOf(text)}
    </span>
  );
}
