// Package registry provides adapter implementations for the port/registry
// interfaces. It includes a local filesystem registry (for dev/testing),
// an HTTP REST registry client (for production), a git-based registry for
// packages distributed as git repositories, a local disk cache, SHA256
// integrity verification, and semver version constraint matching.
package registry
