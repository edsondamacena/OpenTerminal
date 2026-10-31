// Controla o OpenSSH de teste: up | down | rotate-key | restore-key
import { execFileSync } from 'node:child_process';
import { createConnection } from 'node:net';
import { join } from 'node:path';
import { generateTestKeys } from './gen-test-keys.mjs';

const COMPOSE_DIR = join(import.meta.dirname, '..', 'tests', 'fixtures', 'sshd');
export const SSHD = { host: '127.0.0.1', port: 2222 };
/** Só algoritmos legados (tests/fixtures/sshd-legacy). */
export const SSHD_LEGACY = { host: '127.0.0.1', port: 2223 };
/** Proxy que derruba conexões ociosas por 60 s na frente do `sshd` (tests/fixtures/idle-killer). */
export const IDLE_KILLER = { host: '127.0.0.1', port: 2224 };

function compose(...args) {
  return execFileSync('docker', ['compose', ...args], {
    cwd: COMPOSE_DIR,
    stdio: ['ignore', 'pipe', 'pipe'],
    encoding: 'utf8',
  });
}

export function dockerAvailable() {
  try {
    execFileSync('docker', ['info', '--format', '{{.ServerVersion}}'], { stdio: 'pipe' });
    return true;
  } catch {
    return false;
  }
}

/** Espera um servidor de teste responder com o banner SSH. */
export function waitForSshd(timeoutMs = 30_000, target = SSHD) {
  const deadline = Date.now() + timeoutMs;
  return new Promise((resolve, reject) => {
    const attempt = () => {
      const socket = createConnection(target.port, target.host);
      socket.setTimeout(2000);
      socket.once('data', (buf) => {
        socket.destroy();
        if (buf.toString().startsWith('SSH-')) resolve();
        else retry();
      });
      socket.once('error', retry);
      socket.once('timeout', retry);
      function retry() {
        socket.destroy();
        if (Date.now() > deadline)
          reject(new Error(`sshd de teste não respondeu (${target.host}:${target.port})`));
        else setTimeout(attempt, 500);
      }
    };
    attempt();
  });
}

export async function up() {
  if (!dockerAvailable()) {
    throw new Error('Docker não está disponível. Inicie o Docker Desktop e tente novamente.');
  }
  generateTestKeys();
  compose('up', '-d', '--build', '--wait');
  await Promise.all([SSHD, SSHD_LEGACY, IDLE_KILLER].map((t) => waitForSshd(30_000, t)));
}

export function down() {
  compose('down', '--remove-orphans');
}

/** Para o contêiner sem removê-lo (simula queda do servidor). */
export function stop() {
  compose('stop');
}

export async function start() {
  compose('start');
  await waitForSshd();
}

function sshdExec(script) {
  compose('exec', '-T', 'sshd', 'sh', '-c', script);
}

/** Troca a host key e recarrega o sshd (testa divergência de chave). */
export async function rotateKey() {
  sshdExec(
    "rm -f /etc/ssh/ssh_host_ed25519_key* && ssh-keygen -q -t ed25519 -N '' -f /etc/ssh/ssh_host_ed25519_key && pkill -HUP -f sshd.pam",
  );
  await waitForSshd();
}

export async function restoreKey() {
  sshdExec('cp /etc/ssh/original/* /etc/ssh/ && pkill -HUP -f sshd.pam');
  await waitForSshd();
}

const commands = { up, down, stop, start, 'rotate-key': rotateKey, 'restore-key': restoreKey };

if (import.meta.url === `file:///${process.argv[1]?.replaceAll('\\', '/')}`) {
  const cmd = commands[process.argv[2] ?? ''];
  if (!cmd) {
    console.error(`Uso: node scripts/sshd.mjs <${Object.keys(commands).join('|')}>`);
    process.exit(2);
  }
  try {
    await cmd();
    console.log(`sshd de teste: ${process.argv[2]} ok (${SSHD.host}:${SSHD.port})`);
  } catch (e) {
    console.error(e instanceof Error ? e.message : e);
    process.exit(1);
  }
}
