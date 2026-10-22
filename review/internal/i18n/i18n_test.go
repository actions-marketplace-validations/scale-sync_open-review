package i18n

import (
	"testing"
)

func TestI18n_NormalizeAndTranslate(t *testing.T) {
	if Normalize("es-ES") != "es" {
		t.Errorf("expected 'es', got %q", Normalize("es-ES"))
	}
	if Normalize("ja_JP.UTF-8") != "ja" {
		t.Errorf("expected 'ja', got %q", Normalize("ja_JP.UTF-8"))
	}
	if Normalize("fr") != "en" {
		t.Errorf("expected fallback 'en', got %q", Normalize("fr"))
	}

	SetLocale("en")
	msgEn := T("review.complete", 5, "100ms")
	if msgEn != "Review complete: 5 findings (100ms)" {
		t.Errorf("unexpected en translation: %q", msgEn)
	}

	SetLocale("ja")
	msgJa := T("review.complete", 5, "100ms")
	if msgJa != "レビュー完了: 5 件の指摘 (100ms)" {
		t.Errorf("unexpected ja translation: %q", msgJa)
	}

	SetLocale("es")
	msgEs := T("config.connected", "azure", "gpt-4o")
	if msgEs != "Conectado a azure (gpt-4o)" {
		t.Errorf("unexpected es translation: %q", msgEs)
	}

	SetLocale("zh")
	msgZh := T("review.empty")
	if msgZh != "没有需要审查的文件" {
		t.Errorf("unexpected zh translation: %q", msgZh)
	}

	SetLocale("de")
	msgDe := T("review.empty")
	if msgDe != "Keine Dateien zur Überprüfung" {
		t.Errorf("unexpected de translation: %q", msgDe)
	}

	// Unknown key falls back to key itself
	if got := T("unknown.key"); got != "unknown.key" {
		t.Errorf("expected 'unknown.key', got %q", got)
	}
}
