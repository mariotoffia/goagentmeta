package registry_test

import (
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/adapter/registry"
	portregistry "github.com/mariotoffia/goagentmeta/internal/port/registry"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    registry.Version
		wantErr bool
	}{
		{"simple", "1.2.3", registry.Version{Major: 1, Minor: 2, Patch: 3}, false},
		{"v prefix", "v1.2.3", registry.Version{Major: 1, Minor: 2, Patch: 3}, false},
		{"zero", "0.0.0", registry.Version{Major: 0, Minor: 0, Patch: 0}, false},
		{"large", "10.20.300", registry.Version{Major: 10, Minor: 20, Patch: 300}, false},
		{"invalid text", "abc", registry.Version{}, true},
		{"missing patch", "1.2", registry.Version{}, true},
		{"empty", "", registry.Version{}, true},
		{"extra parts", "1.2.3.4", registry.Version{}, true},
		{"negative", "-1.2.3", registry.Version{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := registry.ParseVersion(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseVersion(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseVersion(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestVersion_String(t *testing.T) {
	v := registry.Version{Major: 1, Minor: 2, Patch: 3}
	if got := v.String(); got != "1.2.3" {
		t.Errorf("String() = %q, want %q", got, "1.2.3")
	}
}

func TestVersion_Compare(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want int
	}{
		{"equal", "1.2.3", "1.2.3", 0},
		{"major less", "1.2.3", "2.2.3", -1},
		{"major greater", "2.2.3", "1.2.3", 1},
		{"minor less", "1.2.3", "1.3.3", -1},
		{"minor greater", "1.3.3", "1.2.3", 1},
		{"patch less", "1.2.3", "1.2.4", -1},
		{"patch greater", "1.2.4", "1.2.3", 1},
		{"zeros", "0.0.0", "0.0.0", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, _ := registry.ParseVersion(tt.a)
			b, _ := registry.ParseVersion(tt.b)
			if got := a.Compare(b); got != tt.want {
				t.Errorf("(%s).Compare(%s) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestMatchConstraint(t *testing.T) {
	tests := []struct {
		name       string
		version    string
		constraint string
		want       bool
	}{
		{"wildcard", "1.2.3", "*", true},
		{"empty", "1.2.3", "", true},
		{"exact match", "1.2.3", "1.2.3", true},
		{"exact no match", "1.2.3", "1.2.4", false},

		// caret: >=x.y.z, <(x+1).0.0
		{"caret match same", "1.2.3", "^1.2.3", true},
		{"caret match higher minor", "1.3.0", "^1.2.0", true},
		{"caret match higher patch", "1.2.5", "^1.2.3", true},
		{"caret no match major", "2.0.0", "^1.2.0", false},
		{"caret no match lower", "1.1.0", "^1.2.0", false},
		{"caret zero major minor bump", "0.2.0", "^0.1.0", false},
		{"caret zero major same minor", "0.1.5", "^0.1.0", true},
		{"caret zero zero exact", "0.0.1", "^0.0.1", true},
		{"caret zero zero different patch", "0.0.2", "^0.0.1", false},

		// tilde: >=x.y.z, <x.(y+1).0
		{"tilde match same", "1.2.3", "~1.2.3", true},
		{"tilde match higher patch", "1.2.5", "~1.2.3", true},
		{"tilde no match minor", "1.3.0", "~1.2.3", false},
		{"tilde no match lower", "1.2.2", "~1.2.3", false},

		// comparison operators
		{"gte match", "2.0.0", ">=1.2.3", true},
		{"gte equal", "1.2.3", ">=1.2.3", true},
		{"gte no", "1.2.2", ">=1.2.3", false},
		{"gt match", "1.2.4", ">1.2.3", true},
		{"gt no equal", "1.2.3", ">1.2.3", false},
		{"gt no less", "1.2.2", ">1.2.3", false},
		{"lte match less", "1.2.2", "<=1.2.3", true},
		{"lte match equal", "1.2.3", "<=1.2.3", true},
		{"lte no", "1.2.4", "<=1.2.3", false},
		{"lt match", "1.2.2", "<1.2.3", true},
		{"lt no equal", "1.2.3", "<1.2.3", false},
		{"lt no greater", "1.2.4", "<1.2.3", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := registry.ParseVersion(tt.version)
			if err != nil {
				t.Fatal(err)
			}
			c := portregistry.VersionConstraint{Raw: tt.constraint}
			got, err := registry.MatchConstraint(v, c)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("MatchConstraint(%s, %q) = %v, want %v",
					tt.version, tt.constraint, got, tt.want)
			}
		})
	}
}

func TestMatchConstraint_InvalidConstraint(t *testing.T) {
	v := registry.Version{Major: 1, Minor: 0, Patch: 0}
	_, err := registry.MatchConstraint(v, portregistry.VersionConstraint{Raw: "^invalid"})
	if err == nil {
		t.Error("expected error for invalid constraint, got nil")
	}
}

func TestBestMatch(t *testing.T) {
	candidates := []registry.Version{
		{Major: 1, Minor: 0, Patch: 0},
		{Major: 1, Minor: 2, Patch: 0},
		{Major: 1, Minor: 3, Patch: 5},
		{Major: 2, Minor: 0, Patch: 0},
	}

	tests := []struct {
		name       string
		constraint string
		wantVer    string
		wantFound  bool
	}{
		{"any", "*", "2.0.0", true},
		{"caret 1.x", "^1.0.0", "1.3.5", true},
		{"exact", "1.2.0", "1.2.0", true},
		{"tilde 1.2.x", "~1.2.0", "1.2.0", true},
		{"no match", "3.0.0", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := portregistry.VersionConstraint{Raw: tt.constraint}
			best, found, err := registry.BestMatch(candidates, c)
			if err != nil {
				t.Fatal(err)
			}
			if found != tt.wantFound {
				t.Errorf("BestMatch found = %v, want %v", found, tt.wantFound)
			}
			if found && best.String() != tt.wantVer {
				t.Errorf("BestMatch = %s, want %s", best, tt.wantVer)
			}
		})
	}
}

func TestBestMatch_Empty(t *testing.T) {
	_, found, err := registry.BestMatch(nil, portregistry.VersionConstraint{Raw: "*"})
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Error("expected no match for empty candidates")
	}
}
