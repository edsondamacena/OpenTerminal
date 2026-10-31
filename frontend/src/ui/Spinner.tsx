import { t } from '../i18n/pt-BR';
import s from './ui.module.css';

export function Spinner({ label = t.common.loading }: { label?: string }) {
  return <span className={s.spinner} role="progressbar" aria-label={label} />;
}
