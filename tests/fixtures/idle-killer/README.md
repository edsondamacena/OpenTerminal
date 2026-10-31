# idle-killer

Proxy TCP que derruba conexões sem tráfego por 60 s (`socat -T 60`), como firewalls e NATs
que matam sessões ociosas. Fica na frente do `sshd` de teste, na porta 2224.

Usado pelos testes de keepalive (FR-120, SC-110): com keepalive a cada 30 s a sessão
sobrevive; sem keepalive ela cai. Definido em `tests/fixtures/sshd/compose.yml` e reaproveita
a imagem `openterminal-test-sshd`, que já tem `socat`.
