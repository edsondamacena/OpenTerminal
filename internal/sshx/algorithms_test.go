package sshx

import (
	"crypto/rand"
	"crypto/rsa"
	"reflect"
	"slices"
	"testing"

	"golang.org/x/crypto/ssh"

	"openterminal/internal/apperr"
)

func TestTransportAlgorithmsUseLibraryDefaultsWhenLegacyIsDisabled(t *testing.T) {
	want := ssh.SupportedAlgorithms()
	kex, ciphers, macs := transportAlgorithms(false)

	if !reflect.DeepEqual(kex, want.KeyExchanges) {
		t.Fatalf("key exchanges = %v, want defaults %v", kex, want.KeyExchanges)
	}
	if !reflect.DeepEqual(ciphers, want.Ciphers) {
		t.Fatalf("ciphers = %v, want defaults %v", ciphers, want.Ciphers)
	}
	if !reflect.DeepEqual(macs, want.MACs) {
		t.Fatalf("MACs = %v, want defaults %v", macs, want.MACs)
	}
	if got := hostKeyAlgorithms(false, ""); !reflect.DeepEqual(got, want.HostKeys) {
		t.Fatalf("host key algorithms = %v, want defaults %v", got, want.HostKeys)
	}
}

func TestLegacyAlgorithmsAreAppendedAfterModernDefaults(t *testing.T) {
	defaults := ssh.SupportedAlgorithms()
	kex, ciphers, macs := transportAlgorithms(true)

	wantKex := append(slices.Clone(defaults.KeyExchanges),
		ssh.InsecureKeyExchangeDH14SHA1,
		ssh.InsecureKeyExchangeDH1SHA1,
	)
	wantCiphers := append(slices.Clone(defaults.Ciphers),
		ssh.InsecureCipherAES128CBC,
		ssh.InsecureCipherTripleDESCBC,
	)
	wantHostKeys := append(slices.Clone(defaults.HostKeys), ssh.KeyAlgoRSA)

	if !reflect.DeepEqual(kex, wantKex) {
		t.Fatalf("key exchanges = %v, want %v", kex, wantKex)
	}
	if !reflect.DeepEqual(ciphers, wantCiphers) {
		t.Fatalf("ciphers = %v, want %v", ciphers, wantCiphers)
	}
	if !reflect.DeepEqual(macs, defaults.MACs) {
		t.Fatalf("MACs = %v, want defaults %v", macs, defaults.MACs)
	}
	if got := hostKeyAlgorithms(true, ""); !reflect.DeepEqual(got, wantHostKeys) {
		t.Fatalf("host key algorithms = %v, want %v", got, wantHostKeys)
	}
}

func TestLegacyRSASignerAdvertisesSHA1AfterModernSignatures(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	base, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	signer := (&authState{o: Options{LegacyAlgorithms: true}}).restrict(base)
	multi, ok := signer.(ssh.MultiAlgorithmSigner)
	if !ok {
		t.Fatalf("legacy RSA signer type = %T, want ssh.MultiAlgorithmSigner", signer)
	}
	want := []string{ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256, ssh.KeyAlgoRSA}
	if got := multi.Algorithms(); !reflect.DeepEqual(got, want) {
		t.Fatalf("RSA signature algorithms = %v, want %v", got, want)
	}
}

func TestNoCommonAlgorithmMapsToBridgeErrorCode(t *testing.T) {
	err := &ssh.AlgorithmNegotiationError{
		What:                "key exchange",
		SupportedAlgorithms: []string{"curve25519-sha256"},
		RequestedAlgorithms: []string{ssh.InsecureKeyExchangeDH1SHA1},
	}
	if got := apperr.CodeOf(err); got != apperr.NoCommonAlgorithm {
		t.Fatalf("CodeOf(AlgorithmNegotiationError) = %q, want %q", got, apperr.NoCommonAlgorithm)
	}
}
