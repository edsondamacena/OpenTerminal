package bridge

import (
	"errors"
	"os"
	"strconv"
	"strings"

	"openterminal/internal/apperr"
	"openterminal/internal/domain"
	"openterminal/internal/model"
	"openterminal/internal/tabby"
)

// TabbyImportResult é o resumo mostrado ao usuário; não leva nome, endereço nem senha.
type TabbyImportResult struct {
	Hosts       int `json:"hosts"`
	Telnet      int `json:"telnet"`
	Groups      int `json:"groups"`
	Credentials int `json:"credentials"`
	NoPassword  int `json:"noPassword"`
	Duplicates  int `json:"duplicates"`
	Invalid     int `json:"invalid"`
}

// ImportTabby traz os acessos SSH e telnet do Tabby para o cofre, numa transação só. Rodar de
// novo não duplica: host igual (protocolo, endereço, porta e usuário) é pulado, grupo com o
// mesmo nome no mesmo pai é reaproveitado e senha igual reusa a mesma credencial.
func (s *HostService) ImportTabby() (TabbyImportResult, error) {
	path, err := tabby.DefaultPath()
	if err != nil {
		return TabbyImportResult{}, clean(err)
	}
	lookup, done := tabby.KeyringLookup()
	defer done()
	entries, err := tabby.Read(path, lookup)
	if errors.Is(err, os.ErrNotExist) {
		return TabbyImportResult{}, apperr.New(apperr.NotFound)
	}
	if err != nil {
		return TabbyImportResult{}, apperr.New(apperr.InvalidInput)
	}
	var result TabbyImportResult
	err = s.store.Mutate(func(content *model.Content) error {
		result = importEntries(content, entries, domain.Now())
		return nil
	})
	return result, clean(err)
}

func importEntries(c *model.Content, entries []tabby.Entry, now string) TabbyImportResult {
	var r TabbyImportResult
	groups := map[string]string{} // pai + "\x00" + nome → id
	for _, g := range c.Groups {
		groups[groupKey(g.ParentID, g.Name)] = g.ID
	}
	creds := map[string]string{} // senha → id (credenciais de senha já no cofre)
	names := map[string]bool{}
	for _, cr := range c.Credentials {
		names[cr.Name] = true
		if cr.Kind == model.CredentialPassword {
			if _, ok := creds[cr.Password]; !ok {
				creds[cr.Password] = cr.ID
			}
		}
	}
	existing := map[string]bool{}
	for _, h := range c.Hosts {
		existing[hostKey(string(protocolOf(h)), h.Address, h.Port, h.Username)] = true
	}

	for _, e := range entries {
		address := domain.NormalizeAddress(e.Address)
		key := hostKey(e.Protocol, address, e.Port, e.Username)
		if existing[key] {
			r.Duplicates++
			continue
		}
		input := domain.HostInput{
			Label: truncate(e.Name, 100), Address: address, Port: e.Port, Username: e.Username,
			AuthMethod: model.AuthAsk, Protocol: model.Protocol(e.Protocol), LegacyAlgorithms: e.Legacy,
		}
		// Valida antes de criar grupo e credencial: host recusado não deixa sobra no cofre.
		if !domain.IsValidAddress(address) || e.Port < 1 || e.Port > 65535 || (e.Protocol == "ssh" && e.Username == "") {
			r.Invalid++
			continue
		}
		input.GroupID = ensureGroup(c, groups, e.Group, &r)
		if e.Password != "" && (e.Protocol == "telnet" || e.Username != "") {
			id, ok := creds[e.Password]
			if !ok {
				cr, err := domain.CreatePassword(c, uniqueName(names, "Tabby · "+fallback(e.Username, e.Name)), e.Password, now)
				if err == nil {
					id, ok = cr.ID, true
					creds[e.Password] = id
					r.Credentials++
				}
			}
			if ok {
				input.AuthMethod, input.CredentialID = model.AuthPassword, &id
			}
		}
		if input.AuthMethod == model.AuthAsk {
			r.NoPassword++
		}
		if _, err := domain.CreateHost(c, input, now); err != nil {
			r.Invalid++
			continue
		}
		existing[key] = true
		r.Hosts++
		if e.Protocol == "telnet" {
			r.Telnet++
		}
	}
	return r
}

func ensureGroup(c *model.Content, groups map[string]string, path []string, r *TabbyImportResult) *string {
	var parent *string
	for _, name := range path {
		name = truncate(strings.TrimSpace(name), 100)
		if name == "" {
			continue
		}
		key := groupKey(parent, name)
		id, ok := groups[key]
		if !ok {
			g, err := domain.CreateGroup(c, name, parent)
			if err != nil {
				return parent
			}
			id = g.ID
			groups[key] = id
			r.Groups++
		}
		parent = &id
	}
	return parent
}

func protocolOf(h model.Host) model.Protocol {
	if h.IsTelnet() {
		return model.ProtocolTelnet
	}
	return model.ProtocolSSH
}

func hostKey(protocol, address string, port int, username string) string {
	return protocol + "\x00" + strings.ToLower(address) + "\x00" + strconv.Itoa(port) + "\x00" + username
}

func groupKey(parent *string, name string) string {
	p := ""
	if parent != nil {
		p = *parent
	}
	return p + "\x00" + strings.ToLower(name)
}

func uniqueName(used map[string]bool, base string) string {
	base = truncate(base, 90)
	name := base
	for i := 2; used[name]; i++ {
		name = base + " (" + strconv.Itoa(i) + ")"
	}
	used[name] = true
	return name
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

func fallback(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
