package csvhosts

import "testing"

func TestRead(t *testing.T) {
	data := []byte("\xef\xbb\xbfNome;IP;Porta;Usuário;Protocolo;Grupo;Senha;Legado\n" +
		"OLT Centro;10.0.0.1;;noc;telnet;Provedor / OLTs;x1;\n" +
		"BNG;10.0.0.2;2222;admin;;Provedor;;sim\n" +
		";;;;;;;\n" +
		";10.0.0.3\n")
	got, err := Read(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("%d linhas", len(got))
	}
	olt, bng, last := got[0], got[1], got[2]
	if olt.Protocol != "telnet" || olt.Port != 23 || olt.Password != "x1" || len(olt.Group) != 2 || olt.Group[1] != "OLTs" {
		t.Fatalf("olt: %+v", olt)
	}
	if bng.Protocol != "ssh" || bng.Port != 2222 || !bng.Legacy || bng.Username != "admin" {
		t.Fatalf("bng: %+v", bng)
	}
	if last.Name != "10.0.0.3" || last.Port != 22 {
		t.Fatalf("última: %+v", last)
	}
	if _, err := Read([]byte("nome,usuario\na,b\n")); err != ErrNoAddress {
		t.Fatalf("sem coluna de endereço: %v", err)
	}
	if got, _ := Read([]byte("host,user\n10.0.0.9,root\n")); len(got) != 1 || got[0].Username != "root" {
		t.Fatalf("vírgula: %+v", got)
	}
}
