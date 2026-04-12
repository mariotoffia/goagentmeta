package registry

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	portregistry "github.com/mariotoffia/goagentmeta/internal/port/registry"
)

// Compile-time interface checks.
var (
	_ portregistry.PackageResolver = (*HTTPRegistry)(nil)
	_ portregistry.PackageFetcher  = (*HTTPRegistry)(nil)
	_ portregistry.PackageSearcher = (*HTTPRegistry)(nil)
)

// HTTPRegistry resolves, fetches, and searches packages via an HTTP REST API.
type HTTPRegistry struct {
	baseURL    string
	authToken  string
	client     *http.Client
	maxRetries int
	cacheDir   string
}

// HTTPOption configures an HTTPRegistry.
type HTTPOption func(*HTTPRegistry)

// WithAuthToken sets the authorization bearer token.
func WithAuthToken(token string) HTTPOption {
	return func(r *HTTPRegistry) { r.authToken = token }
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(c *http.Client) HTTPOption {
	return func(r *HTTPRegistry) { r.client = c }
}

// WithMaxRetries sets the maximum number of retry attempts (default 3).
func WithMaxRetries(n int) HTTPOption {
	return func(r *HTTPRegistry) { r.maxRetries = n }
}

// WithCacheDir sets the directory for extracted package archives.
func WithCacheDir(dir string) HTTPOption {
	return func(r *HTTPRegistry) { r.cacheDir = dir }
}

// NewHTTPRegistry creates an HTTP-backed registry client.
func NewHTTPRegistry(baseURL string, opts ...HTTPOption) *HTTPRegistry {
	r := &HTTPRegistry{
		baseURL:    strings.TrimRight(baseURL, "/"),
		client:     &http.Client{Timeout: 30 * time.Second},
		maxRetries: 3,
	}
	for _, opt := range opts {
		opt(r)
	}
	if r.cacheDir == "" {
		r.cacheDir = DefaultCacheDir()
	}
	return r
}

// resolveResponse matches the JSON body from the resolve endpoint.
type resolveResponse struct {
	Name          string `json:"name"`
	Version       string `json:"version"`
	Registry      string `json:"registry"`
	IntegrityHash string `json:"integrity_hash"`
	Publisher     string `json:"publisher"`
}

// Resolve calls GET /api/v1/packages/{name}/versions?constraint={c} to
// resolve the best matching package version.
func (r *HTTPRegistry) Resolve(
	ctx context.Context,
	name string,
	constraint portregistry.VersionConstraint,
) (portregistry.ResolvedPackage, error) {
	u := fmt.Sprintf("%s/api/v1/packages/%s/versions?constraint=%s",
		r.baseURL, url.PathEscape(name), url.QueryEscape(constraint.Raw))

	body, err := r.doGet(ctx, u)
	if err != nil {
		return portregistry.ResolvedPackage{},
			fmt.Errorf("registry: http resolve %q: %w", name, err)
	}
	defer body.Close()

	var resp resolveResponse
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		return portregistry.ResolvedPackage{},
			fmt.Errorf("registry: http resolve %q: decode: %w", name, err)
	}

	return portregistry.ResolvedPackage{
		Name:          resp.Name,
		Version:       resp.Version,
		Registry:      resp.Registry,
		IntegrityHash: resp.IntegrityHash,
		Publisher:     resp.Publisher,
	}, nil
}

// Fetch calls GET /api/v1/packages/{name}/{version}/archive to download a
// tar.gz package archive and extracts it to the cache directory.
func (r *HTTPRegistry) Fetch(
	ctx context.Context,
	pkg portregistry.ResolvedPackage,
) (portregistry.PackageContents, error) {
	u := fmt.Sprintf("%s/api/v1/packages/%s/%s/archive",
		r.baseURL, url.PathEscape(pkg.Name), url.PathEscape(pkg.Version))

	body, err := r.doGet(ctx, u)
	if err != nil {
		return portregistry.PackageContents{},
			fmt.Errorf("registry: http fetch %q@%s: %w", pkg.Name, pkg.Version, err)
	}
	defer body.Close()

	key := CacheKey(pkg.Name, pkg.Version)
	extractDir := filepath.Join(r.cacheDir, key)
	if err := os.RemoveAll(extractDir); err != nil {
		return portregistry.PackageContents{},
			fmt.Errorf("registry: http fetch cleanup: %w", err)
	}
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		return portregistry.PackageContents{},
			fmt.Errorf("registry: http fetch mkdir: %w", err)
	}

	if err := extractTarGz(body, extractDir); err != nil {
		return portregistry.PackageContents{},
			fmt.Errorf("registry: http fetch %q@%s: extract: %w", pkg.Name, pkg.Version, err)
	}

	return portregistry.PackageContents{
		Package: pkg,
		RootDir: extractDir,
	}, nil
}

// Search calls GET /api/v1/search?q={query} to find packages matching the
// query string.
func (r *HTTPRegistry) Search(
	ctx context.Context,
	query string,
) ([]portregistry.PackageMetadata, error) {
	u := fmt.Sprintf("%s/api/v1/search?q=%s",
		r.baseURL, url.QueryEscape(query))

	body, err := r.doGet(ctx, u)
	if err != nil {
		return nil, fmt.Errorf("registry: http search %q: %w", query, err)
	}
	defer body.Close()

	var results []portregistry.PackageMetadata
	if err := json.NewDecoder(body).Decode(&results); err != nil {
		return nil, fmt.Errorf("registry: http search %q: decode: %w", query, err)
	}

	return results, nil
}

// doGet executes an HTTP GET with retry and exponential backoff. It respects
// context cancellation between attempts. A 404 response is not retried.
func (r *HTTPRegistry) doGet(ctx context.Context, rawURL string) (io.ReadCloser, error) {
	var lastErr error

	for attempt := range r.maxRetries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		if r.authToken != "" {
			req.Header.Set("Authorization", "Bearer "+r.authToken)
		}

		resp, err := r.client.Do(req)
		if err != nil {
			lastErr = err
			backoff(ctx, attempt)
			continue
		}

		if resp.StatusCode == http.StatusOK {
			return resp.Body, nil
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("not found (HTTP 404)")
		}

		lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
		if resp.StatusCode >= 500 {
			backoff(ctx, attempt)
			continue
		}

		return nil, lastErr
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// backoff sleeps with exponential backoff, respecting context cancellation.
func backoff(ctx context.Context, attempt int) {
	delay := time.Duration(math.Pow(2, float64(attempt))) * 100 * time.Millisecond
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-timer.C:
	case <-ctx.Done():
	}
}

const (
	// maxFileSize limits individual file extraction to 100 MB.
	maxFileSize = 100 * 1024 * 1024
	// maxTotalSize limits total extracted content to 1 GB.
	maxTotalSize = 1024 * 1024 * 1024
)

// extractTarGz extracts a gzip-compressed tar archive into dst. Path
// traversal is rejected. Individual files are capped at 100 MB and total
// extracted size at 1 GB.
func extractTarGz(r io.Reader, dst string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()

	absDir, err := filepath.Abs(dst)
	if err != nil {
		return fmt.Errorf("abs path: %w", err)
	}

	var totalBytes int64
	tr := tar.NewReader(gz)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}

		name := filepath.Clean(hdr.Name)
		if name == "." {
			continue
		}

		target := filepath.Join(absDir, name)
		if !strings.HasPrefix(target, absDir+string(filepath.Separator)) {
			return fmt.Errorf("tar: path traversal in %q", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if hdr.Size > maxFileSize {
				return fmt.Errorf("tar: file %q exceeds max size (%d > %d)", hdr.Name, hdr.Size, maxFileSize)
			}
			totalBytes += hdr.Size
			if totalBytes > maxTotalSize {
				return fmt.Errorf("tar: total extracted size exceeds limit (%d)", maxTotalSize)
			}

			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(f, io.LimitReader(tr, maxFileSize+1))
			closeErr := f.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		}
	}

	return nil
}
