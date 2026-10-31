package validate

import (
	"strings"
	"testing"

	"openterminal/internal/apperr"
)

func isInvalid(t *testing.T, err error) {
	t.Helper()
	if apperr.CodeOf(err) != apperr.InvalidInput {
		t.Fatalf("esperava INVALID_INPUT, veio %v", err)
	}
}

func TestUUID(t *testing.T) {
	if err := UUID("4ce66518-3068-4bef-ad0b-eb3f0141443d"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"", "x", "4ce6651830684befad0beb3f0141443d", "4ce66518-3068-4bef-ad0b-eb3f0141443", "zzzzzzzz-3068-4bef-ad0b-eb3f0141443d"} {
		isInvalid(t, UUID(bad))
	}
}

func TestOptionalUUID(t *testing.T) {
	if err := OptionalUUID(nil); err != nil {
		t.Fatal(err)
	}
	bad := "nope"
	isInvalid(t, OptionalUUID(&bad))
}

func TestText(t *testing.T) {
	got, err := Text("  OLT Centro  ", 1, 100)
	if err != nil || got != "OLT Centro" {
		t.Fatalf("got %q, %v", got, err)
	}
	_, err = Text("   ", 1, 100)
	isInvalid(t, err)
	// Limite conta caracteres, não bytes.
	if _, err := Text(strings.Repeat("ç", 100), 1, 100); err != nil {
		t.Fatal(err)
	}
	_, err = Text(strings.Repeat("ç", 101), 1, 100)
	isInvalid(t, err)
	_, err = Text("a\x00b", 1, 100)
	isInvalid(t, err)
}

func TestInt(t *testing.T) {
	if err := Int(22, 1, 65535); err != nil {
		t.Fatal(err)
	}
	isInvalid(t, Int(0, 1, 65535))
	isInvalid(t, Int(65536, 1, 65535))
}

func TestOneOf(t *testing.T) {
	if err := OneOf("ssh", "ssh", "local"); err != nil {
		t.Fatal(err)
	}
	isInvalid(t, OneOf("x", "ssh", "local"))
}
