package packager_test

import (
	"context"
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/port/packager"
)

func TestDefaultRegistry_RegisterAndLookup(t *testing.T) {
	reg := packager.NewRegistry()

	p := &stubPackager{format: packager.FormatVSIX}
	if err := reg.Register(p); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, ok := reg.ByFormat(packager.FormatVSIX)
	if !ok {
		t.Fatal("expected to find vsix packager")
	}
	if got.Format() != packager.FormatVSIX {
		t.Errorf("expected format vsix, got %s", got.Format())
	}
}

func TestDefaultRegistry_DuplicateReturnsError(t *testing.T) {
	reg := packager.NewRegistry()

	p1 := &stubPackager{format: packager.FormatNPM}
	p2 := &stubPackager{format: packager.FormatNPM}

	if err := reg.Register(p1); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if err := reg.Register(p2); err == nil {
		t.Fatal("expected error for duplicate registration")
	}
}

func TestDefaultRegistry_MustRegisterPanicsOnDuplicate(t *testing.T) {
	reg := packager.NewRegistry()

	p1 := &stubPackager{format: packager.FormatOCI}
	reg.MustRegister(p1)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for duplicate MustRegister")
		}
	}()
	reg.MustRegister(&stubPackager{format: packager.FormatOCI})
}

func TestDefaultRegistry_ByFormatNotFound(t *testing.T) {
	reg := packager.NewRegistry()

	_, ok := reg.ByFormat(packager.FormatClaudePlugin)
	if ok {
		t.Fatal("expected not found for unregistered format")
	}
}

func TestDefaultRegistry_AllReturnsRegistrationOrder(t *testing.T) {
	reg := packager.NewRegistry()

	reg.MustRegister(&stubPackager{format: packager.FormatVSIX})
	reg.MustRegister(&stubPackager{format: packager.FormatNPM})
	reg.MustRegister(&stubPackager{format: packager.FormatClaudePlugin})

	all := reg.All()
	if len(all) != 3 {
		t.Fatalf("expected 3 packagers, got %d", len(all))
	}

	expected := []packager.Format{packager.FormatVSIX, packager.FormatNPM, packager.FormatClaudePlugin}
	for i, p := range all {
		if p.Format() != expected[i] {
			t.Errorf("position %d: expected %s, got %s", i, expected[i], p.Format())
		}
	}
}

func TestDefaultRegistry_AllReturnsCopy(t *testing.T) {
	reg := packager.NewRegistry()
	reg.MustRegister(&stubPackager{format: packager.FormatVSIX})

	all := reg.All()
	all[0] = nil // Mutate the returned slice

	got, ok := reg.ByFormat(packager.FormatVSIX)
	if !ok || got == nil {
		t.Fatal("mutation of All() result affected registry")
	}
}

// --- stub ---

type stubPackager struct {
	format packager.Format
}

func (s *stubPackager) Format() packager.Format        { return s.format }
func (s *stubPackager) Targets() []build.Target        { return nil }
func (s *stubPackager) Package(_ context.Context, _ packager.PackagerInput) (*packager.PackagerOutput, error) {
	return &packager.PackagerOutput{}, nil
}

// Silence unused import.
var _ = build.TargetClaude
