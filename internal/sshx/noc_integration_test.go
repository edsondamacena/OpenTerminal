//go:build integration && windows

package sshx

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"openterminal/internal/apperr"
	"openterminal/internal/model"
)

const (
	idleKillerPort = 2224
	nocIdleWait    = 150 * time.Second
)

func TestNOCIntegrationLegacyAlgorithms(t *testing.T) {
	auth := Auth{Method: model.AuthPassword, Password: "legacy-test-123"}
	_, err := Dial(context.Background(), integrationOpts(sshdLegacyPort, "legacyuser", newKnown(), auth), &fakeIx{acceptKey: true})
	if got := apperr.CodeOf(err); got != apperr.NoCommonAlgorithm {
		t.Fatalf("legacy disabled: code = %q, want %q (err=%v)", got, apperr.NoCommonAlgorithm, err)
	}

	options := integrationOpts(sshdLegacyPort, "legacyuser", newKnown(), auth)
	options.LegacyAlgorithms = true
	client, err := Dial(context.Background(), options, &fakeIx{acceptKey: true})
	if err != nil {
		t.Fatalf("legacy enabled: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestNOCIntegrationKeepaliveSurvivesIdleProxy(t *testing.T) {
	auth := Auth{Method: model.AuthPassword, Password: "pw-test-123"}

	withKeepalive := integrationOpts(idleKillerPort, "pwuser", newKnown(), auth)
	withKeepalive.Keepalive = 30 * time.Second
	kept, err := Dial(context.Background(), withKeepalive, &fakeIx{acceptKey: true})
	if err != nil {
		t.Fatalf("connect with keepalive: %v", err)
	}
	t.Cleanup(func() { _ = kept.Close() })

	withoutKeepalive := integrationOpts(idleKillerPort, "pwuser", newKnown(), auth)
	idle, err := Dial(context.Background(), withoutKeepalive, &fakeIx{acceptKey: true})
	if err != nil {
		_ = kept.Close()
		t.Fatalf("connect without keepalive: %v", err)
	}
	t.Cleanup(func() { _ = idle.Close() })

	keptDone := make(chan error, 1)
	idleDone := make(chan error, 1)
	go func() { keptDone <- kept.Wait() }()
	go func() { idleDone <- idle.Wait() }()

	started := time.Now()
	deadline := time.NewTimer(nocIdleWait)
	ticker := time.NewTicker(15 * time.Second)
	defer deadline.Stop()
	defer ticker.Stop()
	var idleClosedAt time.Duration
	for {
		select {
		case err := <-keptDone:
			_ = idle.Close()
			if idleClosedAt == 0 {
				<-idleDone
			}
			t.Fatalf("connection with keepalive closed after %v (last RTT %v): %v", time.Since(started), kept.Latency(), err)
		case err := <-idleDone:
			idleClosedAt = time.Since(started)
			idleDone = nil
			t.Logf("connection without keepalive closed after %v: %v", idleClosedAt, err)
		case <-ticker.C:
			t.Logf("elapsed %v, latest keepalive RTT %v", time.Since(started).Round(time.Second), kept.Latency())
		case <-deadline.C:
			goto verify
		}
	}

verify:

	if _, _, err := kept.SendRequest("keepalive@openssh.com", true, nil); err != nil {
		t.Fatalf("connection with keepalive did not survive: %v", err)
	}
	if idleClosedAt == 0 {
		if _, _, err := idle.SendRequest("keepalive@openssh.com", true, nil); err == nil {
			t.Fatal("connection without keepalive survived the idle proxy")
		}
		<-idleDone
	}
	if err := kept.Close(); err != nil {
		t.Fatal(err)
	}
	<-keptDone
	_ = idle.Close()
}

func TestNOCIntegrationWindowsAgent(t *testing.T) {
	privateKey, err := ssh.ParseRawPrivateKey([]byte(readKey(t, "ed25519")))
	if err != nil {
		t.Fatal(err)
	}
	pipe, agentDone := serveTestAgent(t, privateKey)
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("OPENTERMINAL_AGENT_PIPE", pipe)

	client, err := Dial(
		context.Background(),
		integrationOpts(sshdPort, "keyuser", newKnown(), Auth{Method: model.AuthAgent}),
		&fakeIx{acceptKey: true},
	)
	if err != nil {
		t.Fatalf("agent authentication: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	waitForTestAgent(t, agentDone)
}

func TestNOCIntegrationMissingWindowsAgent(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("OPENTERMINAL_AGENT_PIPE", fmt.Sprintf(
		`\\.\pipe\openterminal-agent-missing-integration-%d-%d`, os.Getpid(), time.Now().UnixNano(),
	))

	_, err := Dial(
		context.Background(),
		integrationOpts(sshdPort, "keyuser", newKnown(), Auth{Method: model.AuthAgent}),
		&fakeIx{acceptKey: true},
	)
	if got := apperr.CodeOf(err); got != apperr.AgentUnavailable {
		t.Fatalf("missing agent pipe: code = %q, want %q (err=%v)", got, apperr.AgentUnavailable, err)
	}
}

func waitForTestAgent(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		if err != nil && err != io.EOF {
			t.Logf("agent server stopped after pipe close: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("agent server did not stop after authentication")
	}
}
