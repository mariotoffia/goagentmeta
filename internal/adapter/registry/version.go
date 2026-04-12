package registry

import (
	"fmt"
	"strconv"
	"strings"

	portregistry "github.com/mariotoffia/goagentmeta/internal/port/registry"
)

// Version represents a parsed semantic version (major.minor.patch).
type Version struct {
	Major int
	Minor int
	Patch int
}

// ParseVersion parses a semantic version string. An optional "v" prefix is
// stripped. The string must contain exactly three dot-separated non-negative
// integers (e.g. "1.2.3" or "v1.2.3").
func ParseVersion(s string) (Version, error) {
	s = strings.TrimPrefix(s, "v")
	parts := strings.SplitN(s, ".", 3)
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("registry: invalid version %q: expected major.minor.patch", s)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil || major < 0 {
		return Version{}, fmt.Errorf("registry: invalid major version in %q", s)
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil || minor < 0 {
		return Version{}, fmt.Errorf("registry: invalid minor version in %q", s)
	}

	patch, err := strconv.Atoi(parts[2])
	if err != nil || patch < 0 {
		return Version{}, fmt.Errorf("registry: invalid patch version in %q", s)
	}

	return Version{Major: major, Minor: minor, Patch: patch}, nil
}

// String returns the version in "major.minor.patch" format.
func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// Compare returns -1, 0, or 1 depending on whether v is less than, equal to,
// or greater than other.
func (v Version) Compare(other Version) int {
	if v.Major != other.Major {
		if v.Major < other.Major {
			return -1
		}
		return 1
	}
	if v.Minor != other.Minor {
		if v.Minor < other.Minor {
			return -1
		}
		return 1
	}
	if v.Patch != other.Patch {
		if v.Patch < other.Patch {
			return -1
		}
		return 1
	}
	return 0
}

// MatchConstraint returns true if version v satisfies the given constraint.
// Supported constraint formats:
//
//   - "*" or ""     → any version
//   - "1.2.3"       → exact match
//   - "^1.2.3"      → compatible: >=1.2.3 and <2.0.0 (same major)
//   - "~1.2.3"      → approximate: >=1.2.3 and <1.3.0 (same major.minor)
//   - ">=1.2.3"     → greater than or equal
//   - ">1.2.3"      → strictly greater than
//   - "<=1.2.3"     → less than or equal
//   - "<1.2.3"      → strictly less than
func MatchConstraint(v Version, c portregistry.VersionConstraint) (bool, error) {
	raw := strings.TrimSpace(c.Raw)
	if raw == "" || raw == "*" {
		return true, nil
	}

	switch {
	case strings.HasPrefix(raw, "^"):
		target, err := ParseVersion(raw[1:])
		if err != nil {
			return false, err
		}
		if v.Compare(target) < 0 {
			return false, nil
		}
		// Standard semver caret rules:
		//   ^0.0.z → exact match only
		//   ^0.y.z → same minor (>=0.y.z, <0.(y+1).0)
		//   ^x.y.z → same major (>=x.y.z, <(x+1).0.0)
		if target.Major == 0 {
			if target.Minor == 0 {
				return v.Compare(target) == 0, nil
			}
			return v.Major == 0 && v.Minor == target.Minor, nil
		}
		return v.Major == target.Major, nil

	case strings.HasPrefix(raw, "~"):
		target, err := ParseVersion(raw[1:])
		if err != nil {
			return false, err
		}
		return v.Compare(target) >= 0 &&
			v.Major == target.Major &&
			v.Minor == target.Minor, nil

	case strings.HasPrefix(raw, ">="):
		target, err := ParseVersion(raw[2:])
		if err != nil {
			return false, err
		}
		return v.Compare(target) >= 0, nil

	case strings.HasPrefix(raw, ">") && !strings.HasPrefix(raw, ">="):
		target, err := ParseVersion(raw[1:])
		if err != nil {
			return false, err
		}
		return v.Compare(target) > 0, nil

	case strings.HasPrefix(raw, "<="):
		target, err := ParseVersion(raw[2:])
		if err != nil {
			return false, err
		}
		return v.Compare(target) <= 0, nil

	case strings.HasPrefix(raw, "<") && !strings.HasPrefix(raw, "<="):
		target, err := ParseVersion(raw[1:])
		if err != nil {
			return false, err
		}
		return v.Compare(target) < 0, nil

	default:
		target, err := ParseVersion(raw)
		if err != nil {
			return false, err
		}
		return v.Compare(target) == 0, nil
	}
}

// BestMatch returns the highest version from candidates that satisfies the
// constraint. If no version matches, ok is false.
func BestMatch(candidates []Version, c portregistry.VersionConstraint) (Version, bool, error) {
	var best Version
	found := false

	for _, v := range candidates {
		ok, err := MatchConstraint(v, c)
		if err != nil {
			return Version{}, false, err
		}
		if ok && (!found || v.Compare(best) > 0) {
			best = v
			found = true
		}
	}

	return best, found, nil
}
