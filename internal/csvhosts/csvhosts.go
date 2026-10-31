// Package csvhosts lê acessos de uma planilha exportada em CSV (vírgula ou ponto e vírgula).
// A primeira linha é o cabeçalho; os nomes aceitos estão em columns.
package csvhosts

import (
	"bytes"
	"encoding/csv"
	"errors"
	"strconv"
	"strings"

	"openterminal/internal/tabby"
)

var columns = map[string]string{
	"nome": "name", "name": "name", "label": "name", "descricao": "name", "descrição": "name",
	"endereco": "address", "endereço": "address", "ip": "address", "host": "address", "address": "address",
	"porta": "port", "port": "port",
	"usuario": "user", "usuário": "user", "user": "user", "username": "user", "login": "user",
	"protocolo": "protocol", "protocol": "protocol", "tipo": "protocol",
	"grupo": "group", "group": "group", "pasta": "group",
	"senha": "password", "password": "password",
	"legado": "legacy", "legacy": "legacy",
}

var ErrNoAddress = errors.New("a planilha precisa de uma coluna de endereço (ip, host ou endereço)")

// Read devolve uma entrada por linha. Linhas sem endereço são puladas; o grupo aceita caminho
// separado por "/" ou ">" (ex. "Provedor / OLTs").
func Read(data []byte) ([]tabby.Entry, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	first, _, _ := bytes.Cut(data, []byte("\n"))
	r := csv.NewReader(bytes.NewReader(data))
	if bytes.Count(first, []byte(";")) > bytes.Count(first, []byte(",")) {
		r.Comma = ';'
	}
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true
	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrNoAddress
	}
	index := map[string]int{}
	for i, h := range rows[0] {
		if key, ok := columns[strings.ToLower(strings.TrimSpace(h))]; ok {
			if _, dup := index[key]; !dup {
				index[key] = i
			}
		}
	}
	if _, ok := index["address"]; !ok {
		return nil, ErrNoAddress
	}
	get := func(row []string, key string) string {
		if i, ok := index[key]; ok && i < len(row) {
			return strings.TrimSpace(row[i])
		}
		return ""
	}
	var out []tabby.Entry
	for _, row := range rows[1:] {
		e := tabby.Entry{
			Name: get(row, "name"), Address: get(row, "address"), Username: get(row, "user"),
			Password: get(row, "password"), Protocol: strings.ToLower(get(row, "protocol")),
		}
		if e.Address == "" {
			continue
		}
		if e.Protocol != "telnet" {
			e.Protocol = "ssh"
		}
		e.Port, _ = strconv.Atoi(get(row, "port"))
		if e.Port == 0 {
			e.Port = map[string]int{"ssh": 22, "telnet": 23}[e.Protocol]
		}
		if e.Name == "" {
			e.Name = e.Address
		}
		switch strings.ToLower(get(row, "legacy")) {
		case "sim", "s", "yes", "y", "true", "1", "x":
			e.Legacy = e.Protocol == "ssh"
		}
		for _, part := range strings.FieldsFunc(get(row, "group"), func(r rune) bool { return r == '/' || r == '>' }) {
			if p := strings.TrimSpace(part); p != "" {
				e.Group = append(e.Group, p)
			}
		}
		out = append(out, e)
	}
	return out, nil
}
