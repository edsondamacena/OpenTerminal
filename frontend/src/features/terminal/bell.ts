// Sino do terminal (BEL, FR-177): som curto gerado na hora ou um piscar do quadro.
import type { BellMode } from '@shared/model/types';
import s from './terminal.module.css';

let audio: AudioContext | null = null;
let lastRing = 0;
/** Programas que mandam BEL em rajada (autocompletar sem resultado) tocam uma vez só. */
const MIN_INTERVAL_MS = 150;

function beep(): void {
  audio ??= new AudioContext();
  const osc = audio.createOscillator();
  const gain = audio.createGain();
  osc.frequency.value = 880;
  gain.gain.setValueAtTime(0.06, audio.currentTime);
  gain.gain.exponentialRampToValueAtTime(0.0001, audio.currentTime + 0.12);
  osc.connect(gain).connect(audio.destination);
  osc.start();
  osc.stop(audio.currentTime + 0.12);
}

const FLASH_MS = 140;
const flashTimers = new WeakMap<HTMLElement, ReturnType<typeof setTimeout>>();

/** Liga e desliga por tempo, sem animação: segue visível com movimento reduzido. */
function flash(frame: HTMLElement): void {
  const cls = s.bellFlash ?? '';
  const pending = flashTimers.get(frame);
  if (pending) clearTimeout(pending);
  frame.classList.add(cls);
  flashTimers.set(
    frame,
    setTimeout(() => {
      frame.classList.remove(cls);
    }, FLASH_MS),
  );
}

export function ringBell(mode: BellMode, frame: HTMLElement): void {
  if (mode === 'off') return;
  const now = performance.now();
  if (now - lastRing < MIN_INTERVAL_MS) return;
  lastRing = now;
  if (mode === 'sound') beep();
  else flash(frame);
}
