# OpenSSH que só aceita algoritmos legados (FR-121, SC-111). Simula OLTs e roteadores antigos.
# Nunca usar fora de testes.
FROM alpine:3.22

RUN apk add --no-cache openssh-server \
 && ssh-keygen -t rsa -b 2048 -N '' -f /etc/ssh/ssh_host_rsa_key

COPY sshd_config /etc/ssh/sshd_config
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 22
ENTRYPOINT ["/entrypoint.sh"]
