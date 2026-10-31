package sshx

import (
	"time"

	"openterminal/internal/apperr"
)

const maxKeepaliveFailures = 3

type keepaliveConn interface {
	SendRequest(name string, wantReply bool, payload []byte) (bool, []byte, error)
	Close() error
}

type keepaliveReply struct {
	rtt time.Duration
	err error
}

// runKeepalive envia uma requisição OpenSSH por intervalo. Três erros ou intervalos sem
// resposta consecutivos encerram a conexão. O canal stop pertence ao ciclo de vida da sessão.
func runKeepalive(
	stop <-chan struct{},
	conn keepaliveConn,
	interval time.Duration,
	onLatency func(time.Duration),
) error {
	if interval <= 0 {
		return nil
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	replies := make(chan keepaliveReply, 1)
	inflight := false
	failures := 0
	closeConn := func() {
		_ = conn.Close()
		if inflight {
			// Close deve desbloquear SendRequest. Esperar aqui impede que a goroutine da
			// requisição sobreviva ao ciclo de vida da sessão.
			<-replies
			inflight = false
		}
	}
	fail := func() error {
		closeConn()
		return apperr.New(apperr.SessionClosed)
	}

	for {
		select {
		case <-stop:
			closeConn()
			return nil
		case reply := <-replies:
			inflight = false
			if reply.err != nil {
				failures++
				if failures >= maxKeepaliveFailures {
					return fail()
				}
				continue
			}
			failures = 0
			if onLatency != nil {
				onLatency(reply.rtt)
			}
		case <-ticker.C:
			if inflight {
				failures++
				if failures >= maxKeepaliveFailures {
					return fail()
				}
				continue
			}
			inflight = true
			started := time.Now()
			go func() {
				_, _, err := conn.SendRequest("keepalive@openssh.com", true, nil)
				replies <- keepaliveReply{rtt: time.Since(started), err: err}
			}()
		}
	}
}
