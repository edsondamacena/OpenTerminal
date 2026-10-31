package sshx

import (
	"strings"

	"openterminal/internal/apperr"
	"openterminal/internal/model"
	"openterminal/internal/session"
)

// ask pergunta ao usuário sem contar o tempo de resposta no timeout. nil = cancelado.
func (a *authState) ask(q session.PromptQuestion) ([]string, error) {
	a.dl.pause()
	answers, err := a.ix.AskPrompt(q)
	a.dl.resume()
	if err == nil && answers == nil {
		err = apperr.New(apperr.AuthFailed)
	}
	if err != nil {
		a.failure = err
		return nil, err
	}
	a.ix.SetState(session.StateAuthenticating)
	return answers, nil
}

func (a *authState) keyboardInteractive(name, instruction string, questions []string, echos []bool) ([]string, error) {
	// Rodada sem perguntas (só instruções) não interrompe o usuário.
	if len(questions) == 0 {
		return []string{}, nil
	}
	// Equipamentos de rede costumam pedir a senha só por keyboard-interactive: a senha guardada
	// responde uma vez a um único prompt sem eco.
	if a.o.Auth.Method == model.AuthPassword && !a.usedPassword && len(questions) == 1 && !echos[0] {
		a.usedPassword = true
		return []string{a.o.Auth.Password}, nil
	}
	q := session.PromptQuestion{Name: name, Instructions: instruction}
	for i, text := range questions {
		q.Prompts = append(q.Prompts, session.PromptItem{Text: text, Echo: echos[i]})
	}
	answers, err := a.ask(q)
	if err != nil {
		return nil, err
	}
	if len(answers) != len(questions) {
		a.failure = apperr.New(apperr.InvalidInput)
		return nil, a.failure
	}
	return answers, nil
}

// askPassword: "perguntar ao conectar" num servidor que só aceita senha.
func (a *authState) askPassword() (string, error) {
	host := a.o.Address
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	answers, err := a.ask(session.PromptQuestion{Prompts: []session.PromptItem{
		{Text: "Senha de " + a.o.Username + "@" + host + ":", Echo: false},
	}})
	if err != nil {
		return "", err
	}
	if len(answers) != 1 {
		a.failure = apperr.New(apperr.InvalidInput)
		return "", a.failure
	}
	return answers[0], nil
}
