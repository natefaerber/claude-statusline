package config

import (
	"reflect"
	"testing"
)

func TestEffectiveSeparatorsLineList(t *testing.T) {
	line := Line{Separators: []string{" | ", " · "}}
	cfg := &Config{Separator: "!"}
	got := line.EffectiveSeparators(cfg)
	if !reflect.DeepEqual(got, []string{" | ", " · "}) {
		t.Fatalf("line.Separators should win, got %v", got)
	}
}

func TestEffectiveSeparatorsLineSingle(t *testing.T) {
	line := Line{Separator: " | "}
	cfg := &Config{Separators: []string{" · ", " * "}}
	got := line.EffectiveSeparators(cfg)
	if !reflect.DeepEqual(got, []string{" | "}) {
		t.Fatalf("line.Separator should beat cfg.Separators, got %v", got)
	}
}

func TestEffectiveSeparatorsCfgList(t *testing.T) {
	line := Line{}
	cfg := &Config{Separator: "!", Separators: []string{" · ", " * "}}
	got := line.EffectiveSeparators(cfg)
	if !reflect.DeepEqual(got, []string{" · ", " * "}) {
		t.Fatalf("cfg.Separators should beat cfg.Separator, got %v", got)
	}
}

func TestEffectiveSeparatorsCfgSingle(t *testing.T) {
	line := Line{}
	cfg := &Config{Separator: " | "}
	got := line.EffectiveSeparators(cfg)
	if !reflect.DeepEqual(got, []string{" | "}) {
		t.Fatalf("cfg.Separator should be used, got %v", got)
	}
}

func TestEffectiveSeparatorsDefault(t *testing.T) {
	line := Line{}
	cfg := &Config{}
	got := line.EffectiveSeparators(cfg)
	if !reflect.DeepEqual(got, []string{" │ "}) {
		t.Fatalf("default separator fallback failed, got %v", got)
	}
}
