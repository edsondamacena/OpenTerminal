//go:build e2e

package testsvc

import (
	"os"
	"testing"
)

func TestSetDialogPathChangesTheDialogFile(t *testing.T) {
	t.Setenv("OPENTERMINAL_TEST_DIALOG_PATH", "a.json")
	svc := New(nil, nil)
	if err := svc.SetDialogPath(`C:\tmp\b.itermcolors`); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("OPENTERMINAL_TEST_DIALOG_PATH"); got != `C:\tmp\b.itermcolors` {
		t.Fatalf("OPENTERMINAL_TEST_DIALOG_PATH = %q", got)
	}
}
