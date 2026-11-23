package sshx

import (
	"errors"
	"testing"

	"openterminal/internal/apperr"
	"openterminal/internal/model"
	"openterminal/internal/session"
)

func TestAskUsername(t *testing.T) {
	ix := &fakeIx{answers: []string{"  noc  "}}
	user, err := askUsername("10.0.0.1", 2222, ix)
	if err != nil || user != "noc" {
		t.Fatalf("%q %v", user, err)
	}
	if len(ix.prompts) != 1 || ix.prompts[0].Prompts[0].Text != "Usuário para 10.0.0.1:2222:" || !ix.prompts[0].Prompts[0].Echo {
		t.Fatalf("pergunta: %+v", ix.prompts)
	}

	var ae *apperr.Error
	if _, err := askUsername("10.0.0.1", 22, &fakeIx{answers: nil}); !errors.As(err, &ae) || ae.Code != apperr.AuthFailed {
		t.Fatalf("cancelar: %v", err)
	}
	for _, bad := range [][]string{{""}, {"de ploy"}, {"a", "b"}} {
		if _, err := askUsername("10.0.0.1", 22, &fakeIx{answers: bad}); !errors.As(err, &ae) || ae.Code != apperr.InvalidInput {
			t.Fatalf("%q aceito: %v", bad, err)
		}
	}
}

// seqIx responde cada pergunta com a próxima resposta da fila.
type seqIx struct {
	*fakeIx
	queue [][]string
}

func (s *seqIx) AskPrompt(q session.PromptQuestion) ([]string, error) {
	_, _ = s.fakeIx.AskPrompt(q)
	next := s.queue[0]
	s.queue = s.queue[1:]
	return next, nil
}

func TestDialAsksUsernameWhenEmpty(t *testing.T) {
	s := startServer(t, func(s *server) { s.cfg.KeyboardInteractiveCallback = nil })
	o := opts(s, newKnown(), Auth{Method: model.AuthAsk})
	o.Username = ""
	base := &fakeIx{acceptKey: true}
	ix := &seqIx{fakeIx: base, queue: [][]string{{"u"}, {"pw"}}}
	base.ctx = t.Context()
	c, err := Dial(t.Context(), o, ix)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if len(base.prompts) != 2 || base.prompts[1].Prompts[0].Text != "Senha de u@127.0.0.1:" {
		t.Fatalf("prompts %+v", base.prompts)
	}
}
