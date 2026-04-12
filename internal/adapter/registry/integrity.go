package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	portregistry "github.com/mariotoffia/goagentmeta/internal/port/registry"
)

// Compile-time interface check.
var _ portregistry.IntegrityVerifier = (*SHA256Verifier)(nil)

// SHA256Verifier verifies package integrity by computing a SHA256 hash over
// the package contents directory and comparing it to the declared hash.
type SHA256Verifier struct{}

// NewSHA256Verifier creates a new SHA256-based integrity verifier.
func NewSHA256Verifier() *SHA256Verifier {
	return &SHA256Verifier{}
}

// Verify checks that the contents of the package directory match the
// integrity hash declared in pkg. If pkg.IntegrityHash is empty, the check
// is skipped. On mismatch an error reports both expected and actual hashes.
func (v *SHA256Verifier) Verify(pkg portregistry.ResolvedPackage, contents portregistry.PackageContents) error {
	if pkg.IntegrityHash == "" {
		return nil
	}

	actual, err := ComputeIntegrityHash(contents.RootDir)
	if err != nil {
		return fmt.Errorf("registry: integrity verify %q: %w", pkg.Name, err)
	}

	if actual != pkg.IntegrityHash {
		return fmt.Errorf(
			"registry: integrity mismatch for %q: expected=%s, actual=%s",
			pkg.Name, pkg.IntegrityHash, actual,
		)
	}

	return nil
}

// ComputeIntegrityHash computes a deterministic SHA256 hash over all files
// in rootDir. Files are sorted lexicographically and each entry contributes
// its path (normalized to forward slashes) and content, separated by null
// bytes. The returned string has the format "sha256:<hex>".
func ComputeIntegrityHash(rootDir string) (string, error) {
	var paths []string

	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(rootDir, path)
		if err != nil {
			return err
		}
		paths = append(paths, rel)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("registry: walk %q: %w", rootDir, err)
	}

	sort.Strings(paths)

	h := sha256.New()
	for _, p := range paths {
		normalized := strings.ReplaceAll(p, string(filepath.Separator), "/")
		h.Write([]byte(normalized))
		h.Write([]byte{0})

		data, err := os.ReadFile(filepath.Join(rootDir, p))
		if err != nil {
			return "", fmt.Errorf("registry: read %q: %w", p, err)
		}
		h.Write(data)
		h.Write([]byte{0})
	}

	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}
