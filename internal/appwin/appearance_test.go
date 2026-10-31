package appwin

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeAppearanceReader struct {
	mu    sync.Mutex
	state SystemAppearance
	err   error
}

func (f *fakeAppearanceReader) ReadAppearance() (SystemAppearance, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.state, f.err
}

func (f *fakeAppearanceReader) set(state SystemAppearance) {
	f.mu.Lock()
	f.state, f.err = state, nil
	f.mu.Unlock()
}

func TestAppearanceResolutionAndFallback(t *testing.T) {
	p := DefaultPreferences()
	got := ResolveAppearance(SystemAppearance{TransparencyEnabled: true, AppsUseLightTheme: true}, p)
	if !got.Glass || got.Dark || got.HighContrast {
		t.Fatalf("light glass: %+v", got)
	}
	p.GlassEnabled = false
	if got := ResolveAppearance(SystemAppearance{TransparencyEnabled: true}, p); got.Glass || !got.Dark {
		t.Fatalf("glass preference off: %+v", got)
	}
	p.GlassEnabled = true
	if got := ResolveAppearance(SystemAppearance{TransparencyEnabled: true, HighContrast: true}, p); got.Glass || !got.HighContrast {
		t.Fatalf("high contrast: %+v", got)
	}

	reader := &fakeAppearanceReader{err: errors.New("registry unavailable")}
	monitor := NewAppearanceMonitor(reader, func() (Preferences, error) { return p, nil }, func(Appearance) {})
	if got := monitor.Current(); got.Glass || !got.Dark {
		t.Fatalf("solid fallback: %+v", got)
	}
}

func TestAccentColorABGR(t *testing.T) {
	if got := accentColorHex(0xff563412); got != "#123456" {
		t.Fatalf("accentColorHex = %q", got)
	}
	if !ValidAccent("system") || !ValidAccent("brand-violet") || ValidAccent("") || ValidAccent("Brand Violet") {
		t.Fatal("accent id validation is inconsistent")
	}
}

func TestAppearanceWithoutAccentKeepsOtherSystemValues(t *testing.T) {
	got := appearanceFromValues(1, 1, 0, nil)
	if !got.TransparencyEnabled || !got.AppsUseLightTheme || got.HighContrast || got.SystemAccent != nil {
		t.Fatalf("aparência sem AccentColor: %+v", got)
	}
	accent := uint64(0xff563412)
	got = appearanceFromValues(1, 0, hcfHighContrastOn, &accent)
	if got.SystemAccent == nil || *got.SystemAccent != "#123456" || !got.HighContrast {
		t.Fatalf("aparência com AccentColor: %+v", got)
	}
}

func TestAppearanceMonitorEmitsSystemChanges(t *testing.T) {
	reader := &fakeAppearanceReader{state: SystemAppearance{TransparencyEnabled: true}}
	events := make(chan Appearance, 4)
	monitor := NewAppearanceMonitor(reader, func() (Preferences, error) { return DefaultPreferences(), nil }, func(a Appearance) {
		events <- a
	})
	monitor.interval = 5 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	monitor.Start(ctx)
	defer cancel()

	first := <-events
	if !first.Glass || !first.Dark {
		t.Fatalf("initial: %+v", first)
	}
	reader.set(SystemAppearance{TransparencyEnabled: true, AppsUseLightTheme: true})
	select {
	case changed := <-events:
		if changed.Dark {
			t.Fatalf("changed: %+v", changed)
		}
	case <-time.After(time.Second):
		t.Fatal("appearance change was not emitted")
	}
	accent := "#123456"
	reader.set(SystemAppearance{TransparencyEnabled: true, AppsUseLightTheme: true, SystemAccent: &accent})
	select {
	case changed := <-events:
		if changed.SystemAccent == nil || *changed.SystemAccent != accent {
			t.Fatalf("accent change: %+v", changed)
		}
	case <-time.After(time.Second):
		t.Fatal("accent change was not emitted")
	}
}
