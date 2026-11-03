package sshx

import (
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestPTYModesDeclareBackspaceAsDEL(t *testing.T) {
	modes := ptyModes()
	if modes[ssh.VERASE] != 0x7f {
		t.Fatalf("VERASE = %#x, esperava 0x7f (^?)", modes[ssh.VERASE])
	}
	if modes[ssh.ECHO] != 1 {
		t.Fatal("ECHO precisa continuar ligado")
	}
}
