package bridge

import (
	"errors"
	"io"
	"os"
	"strings"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"openterminal/internal/apperr"
	"openterminal/internal/backup"
	"openterminal/internal/csvhosts"
	"openterminal/internal/domain"
	"openterminal/internal/model"
)

// Backup de até 64 MiB: o cofre de um NOC grande cabe com folga; acima disso é outro arquivo.
const maxImportFile = 64 << 20

// BackupImportResult resume a importação de um .otbackup; só contagens.
type BackupImportResult struct {
	Hosts        int `json:"hosts"`
	Groups       int `json:"groups"`
	Credentials  int `json:"credentials"`
	KnownHosts   int `json:"knownHosts"`
	ForwardRules int `json:"forwardRules"`
	Duplicates   int `json:"duplicates"`
	Invalid      int `json:"invalid"`
}

// ExportBackup grava o .otbackup onde o usuário escolher. Devolve o caminho, ou "" se cancelou.
func (s *HostService) ExportBackup(password string) (string, error) {
	content, err := s.store.Read()
	if err != nil {
		return "", clean(err)
	}
	data, err := backup.Encode(backup.Payload{
		CreatedAt: domain.Now(), Groups: content.Groups, Hosts: content.Hosts, Credentials: content.Credentials,
		KnownHosts: content.KnownHosts, ForwardRules: content.ForwardRules,
	}, password, s.kdf)
	if errors.Is(err, backup.ErrWeak) {
		return "", &apperr.Error{Code: apperr.WeakPassword, Message: err.Error()}
	}
	if err != nil {
		return "", clean(err)
	}
	path, err := shell.SaveFile(s.context(), wruntime.SaveDialogOptions{
		Title:           "Salvar backup do OpenTerminal",
		DefaultFilename: "openterminal-" + time.Now().Format("2006-01-02") + ".otbackup",
		Filters:         []wruntime.FileFilter{{DisplayName: "Backup do OpenTerminal", Pattern: "*.otbackup"}},
	})
	if err != nil || path == "" {
		return "", clean(err)
	}
	if !strings.HasSuffix(path, ".otbackup") {
		path += ".otbackup"
	}
	return path, clean(os.WriteFile(path, data, 0o600))
}

// ImportBackup escolhe o arquivo, abre com a senha e mescla no cofre sem duplicar.
func (s *HostService) ImportBackup(password string) (BackupImportResult, error) {
	data, ok, err := s.pickAndRead("Abrir backup do OpenTerminal", "Backup do OpenTerminal", "*.otbackup")
	if err != nil || !ok {
		return BackupImportResult{}, err
	}
	payload, err := backup.Decode(data, password)
	switch {
	case errors.Is(err, backup.ErrBadPassword):
		return BackupImportResult{}, &apperr.Error{Code: apperr.VaultBadPassword, Message: err.Error()}
	case err != nil:
		return BackupImportResult{}, &apperr.Error{Code: apperr.InvalidInput, Message: err.Error()}
	}
	var result BackupImportResult
	err = s.store.Mutate(func(content *model.Content) error {
		result = mergeBackup(content, payload, domain.Now())
		return nil
	})
	return result, clean(err)
}

// ImportCSV lê uma planilha e importa como o Tabby (mesmas regras de grupo e duplicata).
func (s *HostService) ImportCSV() (TabbyImportResult, error) {
	data, ok, err := s.pickAndRead("Importar planilha de acessos", "Planilha CSV", "*.csv;*.txt")
	if err != nil || !ok {
		return TabbyImportResult{}, err
	}
	entries, err := csvhosts.Read(data)
	if err != nil {
		return TabbyImportResult{}, &apperr.Error{Code: apperr.InvalidInput, Message: err.Error()}
	}
	var result TabbyImportResult
	err = s.store.Mutate(func(content *model.Content) error {
		result = importEntries(content, entries, domain.Now())
		return nil
	})
	return result, clean(err)
}

func (s *HostService) pickAndRead(title, filterName, pattern string) ([]byte, bool, error) {
	path, err := shell.OpenFile(s.context(), wruntime.OpenDialogOptions{
		Title: title, Filters: []wruntime.FileFilter{{DisplayName: filterName, Pattern: pattern}},
	})
	if err != nil {
		return nil, false, clean(err)
	}
	if path == "" {
		return nil, false, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, false, clean(err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxImportFile+1))
	if err != nil {
		return nil, false, clean(err)
	}
	if len(data) > maxImportFile {
		return nil, false, &apperr.Error{Code: apperr.InvalidInput, Message: "Arquivo grande demais (máximo 64 MiB)."}
	}
	return data, true, nil
}

// mergeBackup traz o backup para o cofre: os ids do arquivo são trocados por novos, grupo com o
// mesmo nome no mesmo pai é reaproveitado, credencial com o mesmo segredo é reusada e host igual
// (protocolo, endereço, porta, usuário) é pulado. Tudo passa pelas validações do domínio.
func mergeBackup(c *model.Content, p backup.Payload, now string) BackupImportResult {
	var r BackupImportResult
	groupIDs := map[string]string{}
	existingGroups := map[string]string{}
	for _, g := range c.Groups {
		existingGroups[groupKey(g.ParentID, g.Name)] = g.ID
	}
	// Pais antes dos filhos; o que sobrar (ciclo ou pai ausente) vai para a raiz.
	pending := append([]model.Group{}, p.Groups...)
	for len(pending) > 0 {
		var next []model.Group
		for _, g := range pending {
			var parent *string
			if g.ParentID != nil {
				id, ok := groupIDs[*g.ParentID]
				if !ok {
					next = append(next, g)
					continue
				}
				parent = &id
			}
			mapGroup(c, g, parent, groupIDs, existingGroups, &r)
		}
		if len(next) == len(pending) {
			for _, g := range next {
				mapGroup(c, g, nil, groupIDs, existingGroups, &r)
			}
			break
		}
		pending = next
	}

	credIDs := map[string]string{}
	names := map[string]bool{}
	for _, cr := range c.Credentials {
		names[cr.Name] = true
	}
	for _, cr := range p.Credentials {
		if id, ok := sameSecret(c, cr); ok {
			credIDs[cr.ID] = id
			continue
		}
		name := uniqueName(names, fallback(strings.TrimSpace(cr.Name), "Backup"))
		var created model.Credential
		var err error
		switch cr.Kind {
		case model.CredentialPassword:
			created, err = domain.CreatePassword(c, name, cr.Password, now)
		case model.CredentialKey:
			created, err = domain.ImportKey(c, name, cr.PrivateKey, cr.Passphrase, now)
		default:
			continue
		}
		if err != nil {
			continue
		}
		credIDs[cr.ID] = created.ID
		r.Credentials++
	}

	existingHosts := map[string]bool{}
	for _, h := range c.Hosts {
		existingHosts[hostKey(string(protocolOf(h)), h.Address, h.Port, h.Username)] = true
	}
	hostIDs := map[string]string{}
	for _, h := range p.Hosts {
		key := hostKey(string(protocolOf(h)), domain.NormalizeAddress(h.Address), h.Port, h.Username)
		if existingHosts[key] {
			r.Duplicates++
			continue
		}
		in := domain.HostInput{
			Label: h.Label, Address: h.Address, Port: h.Port, Username: h.Username, AuthMethod: h.AuthMethod,
			LegacyAlgorithms: h.LegacyAlgorithms, Protocol: protocolOf(h), DeviceIcon: h.DeviceIcon,
		}
		if h.GroupID != nil {
			if id, ok := groupIDs[*h.GroupID]; ok {
				in.GroupID = &id
			}
		}
		if h.CredentialID != nil {
			if id, ok := credIDs[*h.CredentialID]; ok {
				in.CredentialID = &id
			} else {
				in.AuthMethod = model.AuthAsk
			}
		}
		created, err := domain.CreateHost(c, in, now)
		if err != nil {
			r.Invalid++
			continue
		}
		existingHosts[key] = true
		hostIDs[h.ID] = created.ID
		r.Hosts++
	}

	known := map[string]bool{}
	for _, k := range c.KnownHosts {
		known[k.Endpoint] = true
	}
	for _, k := range p.KnownHosts {
		if !known[k.Endpoint] && k.Endpoint != "" && k.Fingerprint != "" {
			domain.AcceptHostKey(c, k.Endpoint, k.KeyType, k.Fingerprint, now)
			known[k.Endpoint] = true
			r.KnownHosts++
		}
	}
	for _, f := range p.ForwardRules {
		hostID, ok := hostIDs[f.HostID]
		if !ok {
			continue
		}
		if _, err := domain.CreateForwardRule(c, domain.ForwardRuleInput{
			HostID: hostID, Label: f.Label, BindAddress: f.BindAddress, AllowNonLoopback: f.AllowNonLoopback,
			BindPort: f.BindPort, DestHost: f.DestHost, DestPort: f.DestPort,
		}); err == nil {
			r.ForwardRules++
		}
	}
	return r
}

func mapGroup(c *model.Content, g model.Group, parent *string, ids, existing map[string]string, r *BackupImportResult) {
	key := groupKey(parent, g.Name)
	if id, ok := existing[key]; ok {
		ids[g.ID] = id
		return
	}
	created, err := domain.CreateGroup(c, g.Name, parent)
	if err != nil {
		return
	}
	ids[g.ID] = created.ID
	existing[key] = created.ID
	r.Groups++
}

func sameSecret(c *model.Content, cr model.Credential) (string, bool) {
	for _, have := range c.Credentials {
		if have.Kind != cr.Kind {
			continue
		}
		switch cr.Kind {
		case model.CredentialPassword:
			if have.Password == cr.Password {
				return have.ID, true
			}
		case model.CredentialKey:
			if have.PrivateKey == cr.PrivateKey {
				return have.ID, true
			}
		}
	}
	return "", false
}
