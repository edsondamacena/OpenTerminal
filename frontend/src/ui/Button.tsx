import type { ButtonHTMLAttributes } from 'react';
import s from './ui.module.css';

type Variant = 'default' | 'primary' | 'danger' | 'ghost';

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  iconOnly?: boolean;
}

export function Button({
  variant = 'default',
  iconOnly = false,
  className,
  type = 'button',
  ...rest
}: ButtonProps) {
  const classes = [
    s.button,
    variant !== 'default' ? s[variant] : null,
    iconOnly ? s.icon : null,
    className,
  ]
    .filter(Boolean)
    .join(' ');
  return <button type={type} className={classes} {...rest} />;
}
