package appwin

import "testing"

func testGuard(summary ActiveSummary) (*CloseGuard, *[]ActiveSummary, *int) {
	requests := []ActiveSummary{}
	closes := 0
	guard := NewCloseGuard(
		func() ActiveSummary { return summary },
		func(current ActiveSummary) { requests = append(requests, current) },
		func() { closes++ },
	)
	return guard, &requests, &closes
}

func TestCloseGuardIdleClosesDirectly(t *testing.T) {
	guard, requests, _ := testGuard(ActiveSummary{})
	if guard.Intercept() {
		t.Fatal("fechamento ocioso foi impedido")
	}
	if len(*requests) != 0 {
		t.Fatal("pediu confirmação sem atividade")
	}
}

func TestCloseGuardRequestsConfirmationForActiveWork(t *testing.T) {
	cases := []ActiveSummary{
		{Sessions: 1},
		{Transfers: 2},
		{Forwards: 1},
	}
	for _, summary := range cases {
		guard, requests, closes := testGuard(summary)
		if !guard.Intercept() {
			t.Fatalf("não impediu fechamento com %+v", summary)
		}
		if len(*requests) != 1 || (*requests)[0] != summary {
			t.Fatalf("pedidos %+v", *requests)
		}
		if *closes != 0 {
			t.Fatal("fechou antes da confirmação")
		}
	}
}

func TestCloseGuardCancelAndConfirm(t *testing.T) {
	guard, _, closes := testGuard(ActiveSummary{Sessions: 3})
	if !guard.Intercept() {
		t.Fatal("não interceptou")
	}
	guard.Confirm(false)
	if *closes != 0 || !guard.Intercept() {
		t.Fatal("cancelamento alterou o fechamento")
	}
	guard.Confirm(true)
	if *closes != 1 {
		t.Fatalf("fechamentos = %d", *closes)
	}
	if guard.Intercept() {
		t.Fatal("perguntou novamente depois da confirmação")
	}
}
