package bridge

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"openterminal/internal/apperr"
	"openterminal/internal/fonts"
)

func TestBridgeErrorIncludesOperationAndRedactedDetail(t *testing.T) {
	services := testServices(t)
	services.profileService.listFonts = func() ([]fonts.Font, error) {
		return nil, fmt.Errorf(`varredura falhou: password="segredo-noc"`)
	}
	_, err := services.profileService.ListFonts()
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("tipo de erro = %T", err)
	}
	if appErr.Code != apperr.Internal || appErr.Op != "ProfileService.ListFonts" {
		t.Fatalf("envelope = %+v", appErr)
	}
	if !strings.Contains(appErr.Detail, "varredura falhou") || strings.Contains(appErr.Detail, "segredo-noc") {
		t.Fatalf("detail inseguro = %q", appErr.Detail)
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(appErr.Error()), &envelope); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"code", "message", "op", "detail"} {
		if _, ok := envelope[field]; !ok {
			t.Fatalf("campo %q ausente em %v", field, envelope)
		}
	}
}

func TestRecoveredPanicUsesInternalEnvelopeWithoutStack(t *testing.T) {
	services := testServices(t)
	services.profileService.listFonts = func() ([]fonts.Font, error) {
		panic("falha simulada")
	}
	_, err := services.profileService.ListFonts()
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("tipo de erro = %T", err)
	}
	if appErr.Code != apperr.Internal || appErr.Op != "ProfileService.ListFonts" {
		t.Fatalf("panic = %+v", appErr)
	}
	if appErr.Detail != "falha simulada" || strings.Contains(strings.ToLower(appErr.Detail), "goroutine") {
		t.Fatalf("detail do panic = %q", appErr.Detail)
	}
}
