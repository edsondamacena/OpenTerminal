package telnetx

import (
	"regexp"
	"strings"
	"sync"
	"time"
)

// Prompts de login mais comuns em OLT, switch e roteador (Huawei, ZTE, Furukawa, Parks, MikroTik).
var (
	userPrompt = regexp.MustCompile(`(?i)(login|username|user name|user)\s*:\s*$`)
	passPrompt = regexp.MustCompile(`(?i)password\s*:\s*$`)
	ansi       = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)
)

// loginWindow: depois disso o app não digita mais nada sozinho.
const loginWindow = 60 * time.Second

// autoLogin responde uma vez ao prompt de usuário e uma vez ao de senha. Se o equipamento
// perguntar de novo (senha errada), não insiste: a conta não toma bloqueio por repetição.
type autoLogin struct {
	mu       sync.Mutex
	user     string
	password string
	started  time.Time
	tail     string
	sentUser bool
	sentPass bool
}

func newAutoLogin(user, password string) *autoLogin {
	return &autoLogin{user: user, password: password, started: time.Now()}
}

// see recebe a saída do servidor e devolve o que digitar, ou nil.
func (a *autoLogin) see(data []byte) []byte {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.sentPass || time.Since(a.started) > loginWindow {
		a.tail = ""
		return nil
	}
	a.tail += ansi.ReplaceAllString(string(data), "")
	if len(a.tail) > 256 {
		a.tail = a.tail[len(a.tail)-256:]
	}
	text := strings.TrimRight(a.tail, " \t")
	switch {
	case passPrompt.MatchString(text):
		password := a.password
		a.sentPass, a.tail, a.password = true, "", ""
		return encode([]byte(password + "\r"))
	case userPrompt.MatchString(text) && !a.sentUser && a.user != "":
		a.sentUser, a.tail = true, ""
		return encode([]byte(a.user + "\r"))
	}
	return nil
}
