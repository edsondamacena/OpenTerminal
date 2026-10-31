package vault

import (
	"sort"
	"sync"
	"testing"
	"time"

	"openterminal/internal/model"
)

// fakeClock dispara os timers só quando o teste avança o tempo.
type fakeClock struct {
	mu     sync.Mutex
	now    time.Duration
	timers []*fakeTimer
}

type fakeTimer struct {
	at      time.Duration
	f       func()
	stopped bool
}

func (t *fakeTimer) Stop() bool { t.stopped = true; return true }

func (c *fakeClock) afterFunc(d time.Duration, f func()) Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &fakeTimer{at: c.now + d, f: f}
	c.timers = append(c.timers, t)
	return t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	c.now += d
	var due []*fakeTimer
	rest := c.timers[:0]
	for _, t := range c.timers {
		if !t.stopped && t.at <= c.now {
			due = append(due, t)
		} else if !t.stopped {
			rest = append(rest, t)
		}
	}
	c.timers = rest
	c.mu.Unlock()
	sort.Slice(due, func(i, j int) bool { return due[i].at < due[j].at })
	for _, t := range due {
		t.f()
	}
}

// setupAutoLock: cofre com o bloqueio automático ligado em 15 min (desde 31/10 o padrão é desligado).
func setupAutoLock(t *testing.T) (*Store, *AutoLock, *fakeClock) {
	t.Helper()
	s := openTest(t, t.TempDir())
	if err := s.Create(pw, pw); err != nil {
		t.Fatal(err)
	}
	setAutoLock(t, s, model.Ptr(15))
	clk := &fakeClock{}
	return s, newAutoLock(s, clk.afterFunc), clk
}

func setAutoLock(t *testing.T, s *Store, minutes *int) {
	t.Helper()
	if err := s.Mutate(func(c *model.Content) error { c.Settings.AutoLockMinutes = minutes; return nil }); err != nil {
		t.Fatal(err)
	}
}

func TestAutoLockOffByDefault(t *testing.T) {
	s := openTest(t, t.TempDir())
	if err := s.Create(pw, pw); err != nil {
		t.Fatal(err)
	}
	clk := &fakeClock{}
	newAutoLock(s, clk.afterFunc)
	clk.advance(500 * time.Minute)
	if s.Status().State != StateUnlocked {
		t.Fatal("cofre novo bloqueou sozinho: o padrão é só o cadeado")
	}
}

func TestAutoLock15(t *testing.T) {
	s, _, clk := setupAutoLock(t)
	clk.advance(15*time.Minute - time.Second)
	if s.Status().State != StateUnlocked {
		t.Fatal("bloqueou antes de 15 min")
	}
	clk.advance(2 * time.Second)
	if s.Status().State != StateLocked {
		t.Fatal("não bloqueou após 15 min")
	}
}

func TestAutoLockActivityRestarts(t *testing.T) {
	s, a, clk := setupAutoLock(t)
	clk.advance(10 * time.Minute)
	a.ReportActivity()
	clk.advance(10 * time.Minute)
	if s.Status().State != StateUnlocked {
		t.Fatal("atividade não reiniciou o timer")
	}
	clk.advance(6 * time.Minute)
	if s.Status().State != StateLocked {
		t.Fatal("não bloqueou")
	}
}

func TestAutoLockDisabled(t *testing.T) {
	s, a, clk := setupAutoLock(t)
	setAutoLock(t, s, nil)
	a.Reconfigure()
	clk.advance(500 * time.Minute)
	if s.Status().State != StateUnlocked {
		t.Fatal("bloqueou com o bloqueio automático desativado")
	}
}

func TestAutoLockConfiguredValue(t *testing.T) {
	s, a, clk := setupAutoLock(t)
	setAutoLock(t, s, model.Ptr(1))
	a.Reconfigure()
	clk.advance(time.Minute + time.Second)
	if s.Status().State != StateLocked {
		t.Fatal("não usou o valor configurado")
	}
}

func TestAutoLockRearmsOnUnlock(t *testing.T) {
	s, _, clk := setupAutoLock(t)
	s.Lock()
	clk.advance(30 * time.Minute)
	if err := s.Unlock(pw); err != nil {
		t.Fatal(err)
	}
	clk.advance(14 * time.Minute)
	if s.Status().State != StateUnlocked {
		t.Fatal("timer antigo bloqueou após novo desbloqueio")
	}
	clk.advance(2 * time.Minute)
	if s.Status().State != StateLocked {
		t.Fatal("não rearmou ao desbloquear")
	}
}
