// Package tabby lê os acessos do Tabby (~/.config/tabby/config.yaml) para importar no cofre.
// Senhas vêm de três lugares: options.password no YAML, o chaveiro do sistema (onde o Tabby
// guarda as de SSH como "ssh@host:porta") e os scripts de login do telnet (expect/send).
package tabby

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type config struct {
	Profiles []profile `yaml:"profiles"`
	Groups   []group   `yaml:"groups"`
}

type group struct {
	ID       string `yaml:"id"`
	Name     string `yaml:"name"`
	ParentID string `yaml:"parentGroupId"`
}

type profile struct {
	Name    string  `yaml:"name"`
	Type    string  `yaml:"type"`
	Group   string  `yaml:"group"`
	Options options `yaml:"options"`
}

type options struct {
	Host       string              `yaml:"host"`
	Port       int                 `yaml:"port"`
	User       string              `yaml:"user"`
	Password   string              `yaml:"password"`
	Algorithms map[string][]string `yaml:"algorithms"`
	Scripts    []script            `yaml:"scripts"`
}

type script struct {
	Expect string `yaml:"expect"`
	Send   string `yaml:"send"`
}

// Entry é um acesso pronto para o cofre. Password vazia: o usuário digita ao conectar.
type Entry struct {
	Name     string
	Protocol string // "ssh" ou "telnet"
	Address  string
	Port     int
	Username string
	Password string
	Legacy   bool
	// Group é o caminho a partir da raiz, ex. ["Clientes", "NOC"]; vazio fica na raiz.
	Group []string
}

// Lookup busca a senha que o Tabby guardou no chaveiro (service, account).
type Lookup func(service, account string) (string, bool)

func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "tabby", "config.yaml"), nil
}

var (
	userExpect = regexp.MustCompile(`(?i)login|user`)
	passExpect = regexp.MustCompile(`(?i)pass`)
)

// Legados que o Tabby liga para equipamento antigo; qualquer um deles pede LegacyAlgorithms.
var legacyAlgorithms = []string{"ssh-rsa", "ssh-dss", "diffie-hellman-group1-sha1", "diffie-hellman-group14-sha1",
	"diffie-hellman-group-exchange-sha1", "3des-cbc", "aes128-cbc", "aes192-cbc", "aes256-cbc", "hmac-sha1"}

// Read devolve os acessos SSH e telnet; os outros tipos (shell local, serial) ficam de fora.
func Read(path string, lookup Lookup) ([]Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	groups := map[string]group{}
	for _, g := range cfg.Groups {
		groups[g.ID] = g
	}
	var out []Entry
	for _, p := range cfg.Profiles {
		if p.Type != "ssh" && p.Type != "telnet" {
			continue
		}
		o := p.Options
		e := Entry{
			Name: strings.TrimSpace(p.Name), Protocol: p.Type, Address: strings.TrimSpace(o.Host),
			Port: o.Port, Username: strings.TrimSpace(o.User), Password: o.Password,
			Group: groupPath(groups, p.Group),
		}
		if e.Port == 0 {
			e.Port = map[string]int{"ssh": 22, "telnet": 23}[p.Type]
		}
		if e.Name == "" {
			e.Name = e.Address
		}
		if p.Type == "ssh" {
			e.Legacy = usesLegacy(o.Algorithms)
			if e.Password == "" && lookup != nil {
				if pw, ok := lookup("ssh@"+o.Host+":"+strconv.Itoa(e.Port), o.User); ok {
					e.Password = pw
				}
			}
		} else {
			fromScripts(&e, o.Scripts)
		}
		out = append(out, e)
	}
	return out, nil
}

// fromScripts tira usuário e senha dos scripts de login do telnet (expect "login:" → send).
func fromScripts(e *Entry, scripts []script) {
	for _, s := range scripts {
		switch {
		case s.Send == "":
		case passExpect.MatchString(s.Expect):
			if e.Password == "" {
				e.Password = s.Send
			}
		case userExpect.MatchString(s.Expect):
			if e.Username == "" {
				e.Username = strings.TrimSpace(s.Send)
			}
		}
	}
}

func usesLegacy(algorithms map[string][]string) bool {
	for _, list := range algorithms {
		for _, a := range list {
			for _, l := range legacyAlgorithms {
				if a == l {
					return true
				}
			}
		}
	}
	return false
}

// groupPath sobe pelos pais; um ciclo (arquivo corrompido) para no primeiro repetido.
func groupPath(groups map[string]group, id string) []string {
	var path []string
	seen := map[string]bool{}
	for id != "" && !seen[id] {
		seen[id] = true
		g, ok := groups[id]
		if !ok {
			break
		}
		path = append([]string{strings.TrimSpace(g.Name)}, path...)
		id = g.ParentID
	}
	return path
}
