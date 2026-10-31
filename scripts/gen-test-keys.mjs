// Gera chaves SÓ DE TESTE em tests/fixtures/keys/generated (fora do git).
import { generateKeyPairSync } from 'node:crypto';
import { existsSync, mkdirSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import ssh2 from 'ssh2';

const { utils } = ssh2;
export const KEYS_DIR = join(import.meta.dirname, '..', 'tests', 'fixtures', 'keys', 'generated');
export const TEST_PASSPHRASE = 'kp-test-123';

function save(name, privateKey, publicKey) {
  writeFileSync(join(KEYS_DIR, name), privateKey, { mode: 0o600 });
  writeFileSync(join(KEYS_DIR, `${name}.pub`), `${publicKey.trim()} openterminal-test-${name}\n`);
}

export function generateTestKeys({ force = false } = {}) {
  if (!force && existsSync(join(KEYS_DIR, 'ed25519'))) return KEYS_DIR;
  mkdirSync(KEYS_DIR, { recursive: true });

  const ed = utils.generateKeyPairSync('ed25519');
  save('ed25519', ed.private, ed.public);

  const edp = utils.generateKeyPairSync('ed25519', {
    passphrase: TEST_PASSPHRASE,
    cipher: 'aes256-ctr',
  });
  save('ed25519-passphrase', edp.private, edp.public);

  const ec = utils.generateKeyPairSync('ecdsa', { bits: 256 });
  save('ecdsa-p256', ec.private, ec.public);

  // RSA em PEM (PKCS#1) para cobrir o formato não-OpenSSH.
  const rsa = generateKeyPairSync('rsa', {
    modulusLength: 3072,
    privateKeyEncoding: { type: 'pkcs1', format: 'pem' },
    publicKeyEncoding: { type: 'spki', format: 'pem' },
  });
  const parsed = utils.parseKey(rsa.privateKey);
  if (parsed instanceof Error) throw parsed;
  const pub = Array.isArray(parsed) ? parsed[0] : parsed;
  save('rsa-pem', rsa.privateKey, `ssh-rsa ${pub.getPublicSSH().toString('base64')}`);

  writeFileSync(join(KEYS_DIR, 'invalid'), '-----BEGIN NOTHING-----\nnão é uma chave\n');
  return KEYS_DIR;
}

if (import.meta.url === `file:///${process.argv[1]?.replaceAll('\\', '/')}`) {
  generateTestKeys({ force: process.argv.includes('--force') });
  console.log(`Chaves de teste em ${KEYS_DIR}`);
}
