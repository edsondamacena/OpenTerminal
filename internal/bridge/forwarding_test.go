package bridge

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"openterminal/internal/apperr"
	"openterminal/internal/domain"
	"openterminal/internal/forwarding"
	"openterminal/internal/model"
	"openterminal/internal/session"
	"openterminal/internal/sshx"
)

type promptForwardClient struct {
	closed chan struct{}
	once   sync.Once
}

func newPromptForwardClient() *promptForwardClient {
	return &promptForwardClient{closed: make(chan struct{})}
}

func (c *promptForwardClient) Dial(string, string) (net.Conn, error) { return nil, nil }
func (c *promptForwardClient) Wait() error                           { <-c.closed; return nil }
func (c *promptForwardClient) Close() error {
	c.once.Do(func() { close(c.closed) })
	return nil
}

func freeForwardPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	return port
}

func TestForwardServiceCRUDAndLockedBoundary(t *testing.T) {
	services := testServices(t)
	if err := services.vaultService.Create("senha-teste-1", "senha-teste-1"); err != nil {
		t.Fatal(err)
	}
	host, err := services.hostService.Create(domain.HostInput{Label: "Host", Address: "localhost", Port: 22,
		Username: "user", AuthMethod: model.AuthAsk})
	if err != nil {
		t.Fatal(err)
	}
	input := domain.ForwardRuleInput{HostID: host.ID, BindAddress: "127.0.0.1", BindPort: 15432,
		DestHost: "localhost", DestPort: 5432}
	rule, err := services.forwardService.Create(input)
	if err != nil {
		t.Fatal(err)
	}
	listed, err := services.forwardService.List(host.ID)
	if err != nil || len(listed) != 1 || listed[0].Status.Status != "inactive" {
		t.Fatalf("lista: %+v %v", listed, err)
	}
	input.DestPort = 5433
	updated, err := services.forwardService.Update(rule.ID, input)
	if err != nil || updated.DestPort != 5433 {
		t.Fatalf("update: %+v %v", updated, err)
	}
	if err := services.vaultService.Lock(); err != nil {
		t.Fatal(err)
	}
	if _, err := services.forwardService.List(host.ID); apperr.CodeOf(err) != apperr.VaultLocked {
		t.Fatalf("List deveria exigir cofre: %v", err)
	}
	if _, err := services.forwardService.ListActive(); err != nil {
		t.Fatalf("ListActive com cofre bloqueado: %v", err)
	}
	if err := services.forwardService.Deactivate(rule.ID); err != nil {
		t.Fatalf("Deactivate com cofre bloqueado: %v", err)
	}
	if err := services.vaultService.Unlock("senha-teste-1"); err != nil {
		t.Fatal(err)
	}
	if err := services.forwardService.Remove(rule.ID); err != nil {
		t.Fatal(err)
	}
	listed, err = services.forwardService.List(host.ID)
	if err != nil || len(listed) != 0 {
		t.Fatalf("remoção: %+v %v", listed, err)
	}
}

func TestForwardServiceHostKeyInteraction(t *testing.T) {
	for _, accept := range []bool{true, false} {
		t.Run(map[bool]string{true: "aceitar", false: "recusar"}[accept], func(t *testing.T) {
			services := testServices(t)
			if err := services.vaultService.Create("senha-teste-1", "senha-teste-1"); err != nil {
				t.Fatal(err)
			}
			host, err := services.hostService.Create(domain.HostInput{Label: "Host", Address: "localhost", Port: 22,
				Username: "user", AuthMethod: model.AuthAsk})
			if err != nil {
				t.Fatal(err)
			}
			rule, err := services.forwardService.Create(domain.ForwardRuleInput{HostID: host.ID,
				BindAddress: "127.0.0.1", BindPort: freeForwardPort(t), DestHost: "localhost", DestPort: 80})
			if err != nil {
				t.Fatal(err)
			}

			hostKeys := make(chan session.HostKeyEvent, 1)
			serverPrompts := make(chan session.PromptEvent, 1)
			interactions := session.NewManager(func(event string, payload any) {
				switch event {
				case session.EventHostKey:
					hostKeys <- payload.(session.HostKeyEvent)
				case session.EventPrompt:
					serverPrompts <- payload.(session.PromptEvent)
				}
			})
			services.sessions = interactions
			services.sessionService.sessions = interactions
			var prompts atomic.Int32
			client := newPromptForwardClient()
			services.forwardDial = func(_ context.Context, _ sshx.Options, interactor session.Interactor) (forwarding.Client, error) {
				prompts.Add(1)
				accepted, err := interactor.AskHostKey(session.HostKeyQuestion{
					Kind: "new", KeyType: "ssh-ed25519", Fingerprint: "SHA256:teste",
				})
				if err != nil {
					return nil, err
				}
				if !accepted {
					return nil, apperr.New(apperr.HostKeyRejected)
				}
				answers, err := interactor.AskPrompt(session.PromptQuestion{
					Name: "Autenticação", Prompts: []session.PromptItem{{Text: "Senha:", Echo: false}},
				})
				if err != nil {
					return nil, err
				}
				if len(answers) != 1 || answers[0] != "segredo" {
					return nil, apperr.New(apperr.AuthFailed)
				}
				return client, nil
			}
			services.forwards = forwarding.NewManager(services.connectForward, nil)
			services.forwardService.manager = services.forwards

			activated := make(chan error, 1)
			go func() { activated <- services.forwardService.Activate(rule.ID) }()
			var event session.HostKeyEvent
			select {
			case event = <-hostKeys:
			case <-time.After(3 * time.Second):
				t.Fatal("prompt de chave não foi emitido")
			}
			if event.SessionID == "" || event.Kind != "new" || event.KeyType != "ssh-ed25519" || event.Fingerprint != "SHA256:teste" {
				t.Fatalf("evento = %+v", event)
			}
			if err := services.sessionService.AnswerHostKey(event.SessionID, accept); err != nil {
				t.Fatal(err)
			}
			if accept {
				select {
				case prompt := <-serverPrompts:
					if prompt.SessionID != event.SessionID || prompt.Name != "Autenticação" || len(prompt.Prompts) != 1 || prompt.Prompts[0].Text != "Senha:" {
						t.Fatalf("prompt = %+v", prompt)
					}
					if err := services.sessionService.AnswerPrompt(prompt.SessionID, []string{"segredo"}); err != nil {
						t.Fatal(err)
					}
				case <-time.After(3 * time.Second):
					t.Fatal("prompt de autenticação não foi emitido")
				}
			}
			err = <-activated
			if prompts.Load() != 1 {
				t.Fatalf("prompts = %d", prompts.Load())
			}
			status := services.forwards.Status(rule.ID)
			if accept {
				if err != nil || status.Status != forwarding.StatusActive {
					t.Fatalf("ativação = %v, status = %+v", err, status)
				}
				services.forwards.Deactivate(rule.ID)
			} else if apperr.CodeOf(err) != apperr.HostKeyRejected || status.Status != forwarding.StatusInactive {
				t.Fatalf("recusa = %v, status = %+v", err, status)
			}
		})
	}
}
