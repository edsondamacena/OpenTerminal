package sftpx

import (
	"reflect"
	"testing"
)

const partialTestID = "3f2a9c1e-77aa-4bbb-8ccc-0123456789ab"

func TestPartialName(t *testing.T) {
	if got := PartialName("relatório final.pdf", partialTestID); got != "relatório final.pdf.svpart-3f2a9c1e" {
		t.Fatalf("PartialName = %q", got)
	}
	if !IsPartialName("a.txt.svpart-3f2a9c1e") || IsPartialName("a.txt") || IsPartialName("a.svpart-xyz") {
		t.Fatal("detecção de nome parcial incorreta")
	}
}

func TestTransferStateTransitions(t *testing.T) {
	allowed := [][2]TransferState{
		{TransferQueued, TransferRunning}, {TransferQueued, TransferCancelled},
		{TransferQueued, TransferSkipped}, {TransferQueued, TransferFailed},
		{TransferRunning, TransferCompleted}, {TransferRunning, TransferCancelled},
		{TransferRunning, TransferFailed},
	}
	for _, transition := range allowed {
		if !CanTransition(transition[0], transition[1]) {
			t.Errorf("transição %s -> %s deveria ser permitida", transition[0], transition[1])
		}
	}
	denied := [][2]TransferState{
		{TransferQueued, TransferCompleted}, {TransferRunning, TransferQueued},
		{TransferRunning, TransferSkipped}, {TransferCompleted, TransferFailed},
		{TransferCancelled, TransferRunning}, {TransferSkipped, TransferRunning},
		{TransferFailed, TransferCompleted},
	}
	for _, transition := range denied {
		if CanTransition(transition[0], transition[1]) {
			t.Errorf("transição %s -> %s deveria ser recusada", transition[0], transition[1])
		}
	}
}

func TestPlanConflictChoice(t *testing.T) {
	batch := []TransferRef{
		{ID: "a", State: TransferCompleted}, {ID: "b", State: TransferRunning},
		{ID: "c", State: TransferQueued}, {ID: "d", State: TransferQueued},
	}
	cases := []struct {
		name   string
		choice ConflictChoice
		want   map[string]TransferState
	}{
		{"replace", ConflictReplace, map[string]TransferState{}},
		{"skip", ConflictSkip, map[string]TransferState{"c": TransferSkipped}},
		{"cancelAll", ConflictCancelAll, map[string]TransferState{"c": TransferCancelled, "d": TransferCancelled}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := PlanConflictChoice(batch, "c", tc.choice); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("plano = %+v, esperado %+v", got, tc.want)
			}
		})
	}
}
