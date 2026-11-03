package sshx

import (
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestPTYModesDeclareBackspaceAsDEL(t *testing.T) {
	modes := ptyModes(NewTerminal(nil).erase)
	if modes[ssh.VERASE] != 0x7f {
		t.Fatalf("VERASE = %#x, esperava 0x7f (^?)", modes[ssh.VERASE])
	}
	if modes[ssh.ECHO] != 1 {
		t.Fatal("ECHO precisa continuar ligado")
	}
}

func TestPTYModesFollowCtrlH(t *testing.T) {
	term := NewTerminal(nil).WithErase(0x08)
	if modes := ptyModes(term.erase); modes[ssh.VERASE] != 0x08 {
		t.Fatalf("VERASE = %#x, esperava 0x08 (^H)", modes[ssh.VERASE])
	}
	if NewTerminal(nil).WithErase('x').erase != 0x7f {
		t.Fatal("byte inválido não pode trocar o VERASE")
	}
}
