package domain

import (
	"reflect"
	"testing"

	"openterminal/internal/apperr"
	"openterminal/internal/model"
)

const missingForwardID = "00000000-0000-4000-8000-0000000000ff"

func setupForwardRules(t *testing.T) (*model.Content, string, string) {
	t.Helper()
	c := testContent()
	input := baseInput()
	input.AuthMethod, input.CredentialID = model.AuthAsk, nil
	input.Label = "a"
	a, err := CreateHost(c, input, t0)
	if err != nil {
		t.Fatal(err)
	}
	input.Label = "b"
	b, err := CreateHost(c, input, t0)
	if err != nil {
		t.Fatal(err)
	}
	return c, a.ID, b.ID
}

func forwardInput(hostID string) ForwardRuleInput {
	return ForwardRuleInput{HostID: hostID, BindAddress: "127.0.0.1", BindPort: 15432, DestHost: "db.internal", DestPort: 5432}
}

func TestForwardRuleDefaultsAndOptionalLabel(t *testing.T) {
	c, hostID, _ := setupForwardRules(t)
	in := forwardInput(hostID)
	in.BindAddress, in.Label = "  ", model.Ptr("  ")
	rule, err := CreateForwardRule(c, in)
	if err != nil {
		t.Fatal(err)
	}
	if rule.BindAddress != "127.0.0.1" || rule.Label != nil || rule.BindPort != 15432 || len(rule.ID) != 36 {
		t.Fatalf("regra criada incorretamente: %+v", rule)
	}
	in = forwardInput(hostID)
	in.BindPort, in.Label = 15433, model.Ptr(" banco ")
	rule, err = CreateForwardRule(c, in)
	if err != nil || rule.Label == nil || *rule.Label != "banco" {
		t.Fatalf("rótulo não normalizado: %+v %v", rule, err)
	}
}

func TestForwardRuleNonLoopbackRequiresOptIn(t *testing.T) {
	c, hostID, _ := setupForwardRules(t)
	in := forwardInput(hostID)
	in.BindAddress = "0.0.0.0"
	_, err := CreateForwardRule(c, in)
	wantCode(t, err, apperr.InvalidInput)
	in.AllowNonLoopback = true
	if rule, err := CreateForwardRule(c, in); err != nil || rule.BindAddress != "0.0.0.0" {
		t.Fatalf("opt-in não aceito: %+v %v", rule, err)
	}
	for i, address := range []string{"127.0.0.1", "127.5.5.5", "::1", "localhost"} {
		in := forwardInput(hostID)
		in.BindAddress, in.BindPort = address, 20_000+i
		if rule, err := CreateForwardRule(c, in); err != nil || rule.BindAddress != address {
			t.Errorf("loopback %q recusado: %+v %v", address, rule, err)
		}
	}
}

func TestForwardRuleRejectsInvalidInput(t *testing.T) {
	cases := []struct {
		name string
		bad  func(*ForwardRuleInput)
	}{
		{"bindPort zero", func(in *ForwardRuleInput) { in.BindPort = 0 }},
		{"bindPort alto", func(in *ForwardRuleInput) { in.BindPort = 65536 }},
		{"destPort zero", func(in *ForwardRuleInput) { in.DestPort = 0 }},
		{"destPort alto", func(in *ForwardRuleInput) { in.DestPort = 70000 }},
		{"destHost vazio", func(in *ForwardRuleInput) { in.DestHost = " " }},
		{"destHost inválido", func(in *ForwardRuleInput) { in.DestHost = "não é host" }},
		{"bindAddress inválido", func(in *ForwardRuleInput) { in.BindAddress, in.AllowNonLoopback = "abc def", true }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, hostID, _ := setupForwardRules(t)
			in := forwardInput(hostID)
			tc.bad(&in)
			_, err := CreateForwardRule(c, in)
			wantCode(t, err, apperr.InvalidInput)
		})
	}
}

func TestForwardRuleBindPortUniquePerAddress(t *testing.T) {
	c, hostID, otherHostID := setupForwardRules(t)
	if _, err := CreateForwardRule(c, forwardInput(hostID)); err != nil {
		t.Fatal(err)
	}
	_, err := CreateForwardRule(c, forwardInput(otherHostID))
	wantCode(t, err, apperr.InvalidInput)
	in := forwardInput(otherHostID)
	in.BindAddress = "127.0.0.2"
	if _, err := CreateForwardRule(c, in); err != nil {
		t.Fatal(err)
	}
}

func TestForwardRuleHostMustExist(t *testing.T) {
	c, _, _ := setupForwardRules(t)
	_, err := CreateForwardRule(c, forwardInput(missingForwardID))
	wantCode(t, err, apperr.InvalidInput)
}

func TestForwardRuleUpdateRemoveAndList(t *testing.T) {
	c, hostID, otherHostID := setupForwardRules(t)
	a, err := CreateForwardRule(c, forwardInput(hostID))
	if err != nil {
		t.Fatal(err)
	}
	other := forwardInput(otherHostID)
	other.BindPort = 18080
	if _, err := CreateForwardRule(c, other); err != nil {
		t.Fatal(err)
	}
	updated := forwardInput(hostID)
	updated.DestPort = 5433
	got, err := UpdateForwardRule(c, a.ID, updated)
	if err != nil || got.ID != a.ID || got.DestPort != 5433 {
		t.Fatalf("atualização incorreta: %+v %v", got, err)
	}
	updated.BindPort = 18080
	_, err = UpdateForwardRule(c, a.ID, updated)
	wantCode(t, err, apperr.InvalidInput)
	if ids := forwardRuleIDs(ListForwardRulesForHost(c, hostID)); !reflect.DeepEqual(ids, []string{a.ID}) {
		t.Fatalf("lista por host: %v", ids)
	}
	if err := RemoveForwardRule(c, a.ID); err != nil {
		t.Fatal(err)
	}
	if got := ListForwardRulesForHost(c, hostID); len(got) != 0 {
		t.Fatalf("regra não removida: %+v", got)
	}
	wantCode(t, RemoveForwardRule(c, a.ID), apperr.NotFound)
	_, err = UpdateForwardRule(c, a.ID, forwardInput(hostID))
	wantCode(t, err, apperr.NotFound)
}

func forwardRuleIDs(rules []model.ForwardRule) []string {
	ids := make([]string, len(rules))
	for i := range rules {
		ids[i] = rules[i].ID
	}
	return ids
}
