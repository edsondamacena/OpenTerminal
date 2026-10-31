#!/bin/sh
# Cria os usuários de teste (credenciais só de teste, documentadas em quickstart.md).
set -eu

mkuser() {
  id "$1" >/dev/null 2>&1 || adduser -D -s /bin/sh "$1"
  echo "$1:$2" | chpasswd
}

mkuser pwuser pw-test-123
mkuser keyuser "unused-$(head -c 12 /dev/urandom | od -An -tx1 | tr -d ' \n')"
mkuser kbuser kb-test-123

mkdir -p /home/keyuser/.ssh
cat /keys/*.pub > /home/keyuser/.ssh/authorized_keys 2>/dev/null || true
chown -R keyuser:keyuser /home/keyuser/.ssh
chmod 700 /home/keyuser/.ssh
chmod 600 /home/keyuser/.ssh/authorized_keys

exec /usr/sbin/sshd.pam -D -e
