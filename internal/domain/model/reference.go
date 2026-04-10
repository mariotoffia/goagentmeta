package model

// Reference is a supplemental knowledge document that provides in-depth
// material on a topic. References are demand-loaded: the AI reads them only
// when deeper knowledge is needed during a workflow, rather than being
// injected into context unconditionally.
//
// References are distinct from assets: an asset is a static file consumed by
// tooling or emitted into output. A reference is a knowledge document consumed
// by the AI model at read time.
type Reference struct {
	// Path is the relative path to the reference document.
	Path string
	// Description is a brief summary of the reference content.
	Description string
}

// Asset is a static file used by other objects (templates, examples, diagrams,
// prompt partials, sample outputs). Assets may be consumed by tooling or
// emitted into the build output.
type Asset struct {
	// Path is the relative path to the asset file.
	Path string
	// Description is a brief summary of the asset.
	Description string
}

// Script is an executable artifact used by hooks, commands, skills, or plugins.
// Scripts may be validation scripts, resource generators, packaging helpers,
// or local servers used by MCP or plugin runtimes.
type Script struct {
	// Path is the relative path to the script file.
	Path string
	// Description is a brief summary of the script's purpose.
	Description string
	// Interpreter is the runtime used to execute the script (e.g., "bash", "python3").
	// Empty means auto-detect from shebang or file extension.
	Interpreter string
}
