package cloudsync

import "openterminal/internal/model"

// Merge junta o cofre local (com mudanças ainda não enviadas) e o da nuvem (mais novo que a
// última sincronização). Nada some: itens de um lado só entram no resultado. Mesmo ID dos dois
// lados: host fica com o updatedAt mais novo; o resto fica com o local, que é onde a mudança
// pendente foi feita. Limite conhecido: uma exclusão feita só de um lado volta a aparecer.
func Merge(local, cloud Snapshot) Snapshot {
	return Snapshot{
		Settings:     local.Settings,
		Groups:       union(local.Groups, cloud.Groups, func(g model.Group) string { return g.ID }, preferLocal[model.Group]),
		Hosts:        union(local.Hosts, cloud.Hosts, func(h model.Host) string { return h.ID }, newerHost),
		Credentials:  union(local.Credentials, cloud.Credentials, func(c model.Credential) string { return c.ID }, preferLocal[model.Credential]),
		KnownHosts:   union(local.KnownHosts, cloud.KnownHosts, func(k model.KnownHost) string { return k.ID }, preferLocal[model.KnownHost]),
		ForwardRules: union(local.ForwardRules, cloud.ForwardRules, func(r model.ForwardRule) string { return r.ID }, preferLocal[model.ForwardRule]),
	}
}

func preferLocal[T any](local, _ T) T { return local }

// ISO 8601 em UTC com o mesmo formato compara como texto.
func newerHost(local, cloud model.Host) model.Host {
	if cloud.UpdatedAt > local.UpdatedAt {
		return cloud
	}
	return local
}

// union mantém a ordem do local e acrescenta, na ordem da nuvem, o que só existe lá.
func union[T any](local, cloud []T, id func(T) string, pick func(local, cloud T) T) []T {
	fromCloud := make(map[string]T, len(cloud))
	for _, c := range cloud {
		fromCloud[id(c)] = c
	}
	out := make([]T, 0, len(local)+len(cloud))
	seen := make(map[string]bool, len(local))
	for _, l := range local {
		k := id(l)
		seen[k] = true
		if c, ok := fromCloud[k]; ok {
			out = append(out, pick(l, c))
		} else {
			out = append(out, l)
		}
	}
	for _, c := range cloud {
		if !seen[id(c)] {
			out = append(out, c)
		}
	}
	return out
}
