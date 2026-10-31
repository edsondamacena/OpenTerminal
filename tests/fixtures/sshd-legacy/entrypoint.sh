#!/bin/sh
# Usuário de teste do servidor legado (credencial só de teste).
set -eu

id legacyuser >/dev/null 2>&1 || adduser -D -s /bin/sh legacyuser
echo 'legacyuser:legacy-test-123' | chpasswd

mkdir -p /home/legacyuser/.ssh
cat /keys/rsa-pem.pub > /home/legacyuser/.ssh/authorized_keys 2>/dev/null || true
chown -R legacyuser:legacyuser /home/legacyuser/.ssh
chmod 700 /home/legacyuser/.ssh
chmod 600 /home/legacyuser/.ssh/authorized_keys 2>/dev/null || true

exec /usr/sbin/sshd -D -e
