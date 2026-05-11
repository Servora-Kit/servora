package authn

import (
	"context"
	"testing"
)

func TestSubjectFromAny_FirstHit(t *testing.T) {
	fn1 := func(_ context.Context) (string, bool) { return "user-1", true }
	fn2 := func(_ context.Context) (string, bool) { return "user-2", true }

	combined := SubjectFromAny(fn1, fn2)
	got, ok := combined(context.Background())
	if !ok {
		t.Fatal("SubjectFromAny returned ok=false, want true (first hit)")
	}
	if got != "user-1" {
		t.Errorf("SubjectFromAny = %q, want user-1 (first hit wins)", got)
	}
}

func TestSubjectFromAny_SecondHit(t *testing.T) {
	fn1 := func(_ context.Context) (string, bool) { return "", false }
	fn2 := func(_ context.Context) (string, bool) { return "user-2", true }

	combined := SubjectFromAny(fn1, fn2)
	got, ok := combined(context.Background())
	if !ok {
		t.Fatal("SubjectFromAny returned ok=false, want true")
	}
	if got != "user-2" {
		t.Errorf("SubjectFromAny = %q, want user-2", got)
	}
}

func TestSubjectFromAny_AllMiss(t *testing.T) {
	fn1 := func(_ context.Context) (string, bool) { return "", false }
	fn2 := func(_ context.Context) (string, bool) { return "", false }

	combined := SubjectFromAny(fn1, fn2)
	got, ok := combined(context.Background())
	if ok {
		t.Error("SubjectFromAny returned ok=true, want false (all miss)")
	}
	if got != "" {
		t.Errorf("SubjectFromAny = %q, want empty", got)
	}
}

func TestSubjectFromAny_NilFnsSkipped(t *testing.T) {
	fn := func(_ context.Context) (string, bool) { return "found", true }

	combined := SubjectFromAny(nil, nil, fn, nil)
	got, ok := combined(context.Background())
	if !ok {
		t.Fatal("SubjectFromAny returned ok=false, want true")
	}
	if got != "found" {
		t.Errorf("SubjectFromAny = %q, want found", got)
	}
}

func TestSubjectFromAny_Empty(t *testing.T) {
	combined := SubjectFromAny()
	got, ok := combined(context.Background())
	if ok {
		t.Error("SubjectFromAny() returned ok=true with no fns")
	}
	if got != "" {
		t.Errorf("SubjectFromAny() = %q, want empty", got)
	}
}
