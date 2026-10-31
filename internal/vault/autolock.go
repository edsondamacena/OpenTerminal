package vault

import (
	"sync"
	"time"
)

// Timer é o mínimo de time.Timer que o AutoLock usa (os testes trocam o relógio).
type Timer interface{ Stop() bool }

// AutoLock bloqueia o cofre após autoLockMinutes sem atividade (FR-003 da 001). Não encerra
// sessões. autoLockMinutes: "1–120; null = desativado". Padrão desde 31/10: desativado (só o cadeado bloqueia).
type AutoLock struct {
	mu        sync.Mutex
	store     *Store
	afterFunc func(time.Duration, func()) Timer
	timer     Timer
	gen       int
}

func NewAutoLock(store *Store) *AutoLock {
	return newAutoLock(store, func(d time.Duration, f func()) Timer { return time.AfterFunc(d, f) })
}

func newAutoLock(store *Store, afterFunc func(time.Duration, func()) Timer) *AutoLock {
	a := &AutoLock{store: store, afterFunc: afterFunc}
	store.OnStatus(func(st Status) {
		if st.State == StateUnlocked {
			a.Reconfigure()
		} else {
			a.clear()
		}
	})
	if store.Status().State == StateUnlocked {
		a.Reconfigure()
	}
	return a
}

// Reconfigure reinicia o timer com o valor atual (chamar após mudar a configuração).
func (a *AutoLock) Reconfigure() {
	c, err := a.store.Read()
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stopLocked()
	if err != nil || c.Settings.AutoLockMinutes == nil {
		return
	}
	a.gen++
	gen := a.gen
	a.timer = a.afterFunc(time.Duration(*c.Settings.AutoLockMinutes)*time.Minute, func() {
		a.mu.Lock()
		current := a.gen == gen && a.timer != nil
		if current {
			a.timer = nil
		}
		a.mu.Unlock()
		if current {
			a.store.Lock()
		}
	})
}

// ReportActivity reinicia a contagem se o bloqueio automático estiver armado.
func (a *AutoLock) ReportActivity() {
	a.mu.Lock()
	armed := a.timer != nil
	a.mu.Unlock()
	if armed {
		a.Reconfigure()
	}
}

func (a *AutoLock) clear() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stopLocked()
}

func (a *AutoLock) stopLocked() {
	if a.timer != nil {
		a.timer.Stop()
		a.timer = nil
	}
	a.gen++
}
