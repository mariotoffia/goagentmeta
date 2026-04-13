package marketplace

// SourceType identifies how a plugin is fetched. It is a semi-open string enum:
// well-known values are provided as constants, but custom values are permitted
// to support future source types (e.g., OCI registries, S3) without code changes.
type SourceType string

const (
	// SourceRelativePath is a local directory within the marketplace repository.
	// The Location field holds the relative path (must start with "./").
	SourceRelativePath SourceType = "relative"

	// SourceGitHub fetches from a GitHub repository. The Location field holds
	// the "owner/repo" string.
	SourceGitHub SourceType = "github"

	// SourceGitURL fetches from any git repository URL. The Location field
	// holds the full git URL (https:// or git@).
	SourceGitURL SourceType = "url"

	// SourceGitSubdir fetches a subdirectory from a git repository using
	// sparse/partial clone. The Location field holds the repository URL and
	// the Path field holds the subdirectory within the repo.
	SourceGitSubdir SourceType = "git-subdir"

	// SourceNPM installs from an npm registry. The Package field holds the
	// npm package name (e.g., "@acme/claude-plugin").
	SourceNPM SourceType = "npm"
)

// Source describes where to fetch a plugin. The Type field determines which
// other fields are relevant.
type Source struct {
	// Type identifies the source kind.
	Type SourceType

	// Location holds the primary identifier. Its meaning depends on Type:
	//   - relative: relative path (e.g., "./plugins/formatter")
	//   - github:   "owner/repo" (e.g., "company/deploy-plugin")
	//   - url:      full git URL (e.g., "https://gitlab.com/team/plugin.git")
	//   - git-subdir: repository URL
	//   - npm:      unused (use Package instead)
	Location string

	// Ref pins to a specific git branch or tag. Used by github, url, and
	// git-subdir source types.
	Ref string

	// SHA pins to a specific git commit (full 40-character hex). Used by
	// github, url, and git-subdir source types.
	SHA string

	// Path is the subdirectory within a git repository. Used only by
	// git-subdir source type.
	Path string

	// Package is the npm package name (e.g., "@acme/claude-plugin"). Used
	// only by npm source type.
	Package string

	// Version is the npm version or version range (e.g., "2.1.0", "^2.0.0").
	// Used only by npm source type.
	Version string

	// Registry is a custom npm registry URL. Used only by npm source type.
	// Defaults to the system npm registry (npmjs.org) if empty.
	Registry string

	// Extra holds forward-compatible fields for future source types or
	// source-type-specific extensions.
	Extra map[string]string
}
