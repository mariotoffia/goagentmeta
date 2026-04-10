package pipeline

import (
	"time"

	"github.com/mariotoffia/goagentmeta/internal/domain/build"
	"github.com/mariotoffia/goagentmeta/internal/domain/model"
)

// EmissionPlan is a concrete list of outputs the compiler will produce.
// Only after the emission plan is complete should the compiler write files.
type EmissionPlan struct {
	// Units maps build unit output dirs to their emission plan.
	Units map[string]UnitEmission
}

// UnitEmission holds the emission plan for a single build unit.
type UnitEmission struct {
	// Coordinate identifies the build unit.
	Coordinate build.BuildCoordinate

	// Files lists the files to generate.
	Files []EmittedFile

	// Directories lists directories to create.
	Directories []string

	// Assets lists asset files to copy or symlink.
	Assets []EmittedAsset

	// Scripts lists script files to copy or symlink.
	Scripts []EmittedScript

	// PluginBundles lists plugin bundles to package.
	PluginBundles []EmittedPlugin

	// InstallMetadata holds target-native install/config references.
	InstallMetadata []InstallEntry
}

// EmissionLayer identifies whether an emitted file belongs to the instruction
// layer (model-facing text) or the extension layer (runtime configuration).
type EmissionLayer string

const (
	// LayerInstruction is the instruction layer: model-facing text files
	// (CLAUDE.md, AGENTS.md, .cursor/rules/*.mdc, copilot-instructions.md).
	LayerInstruction EmissionLayer = "instruction"
	// LayerExtension is the extension layer: runtime configuration files
	// (.claude/settings.json, .cursor/mcp.json, .vscode/mcp.json).
	LayerExtension EmissionLayer = "extension"
)

// EmittedFile describes a single file to be generated.
type EmittedFile struct {
	// Path is the relative path within the build unit output directory.
	Path string
	// Content is the rendered file content.
	Content []byte
	// Layer identifies whether this is an instruction or extension layer file.
	Layer EmissionLayer
	// SourceObjects lists the canonical object IDs that contributed to this file.
	SourceObjects []string
}

// EmittedAsset describes an asset file to copy or symlink into the output.
type EmittedAsset struct {
	// SourcePath is the relative path in the source tree.
	SourcePath string
	// DestPath is the relative path in the build output.
	DestPath string
}

// EmittedScript describes a script to copy into the output.
type EmittedScript struct {
	// SourcePath is the relative path in the source tree.
	SourcePath string
	// DestPath is the relative path in the build output.
	DestPath string
}

// EmittedPlugin describes a plugin bundle to package in the output.
type EmittedPlugin struct {
	// PluginID is the canonical plugin ID.
	PluginID string
	// DestDir is the output directory for the plugin bundle.
	DestDir string
	// Files lists the files in the plugin bundle.
	Files []EmittedFile
}

// InstallEntry describes a target-native install/config reference
// (e.g., an MCP server entry in settings.json).
type InstallEntry struct {
	// PluginID is the canonical plugin ID being referenced.
	PluginID string
	// Format is the target-native format (e.g., "mcp-server-ref").
	Format string
	// Config holds the target-native configuration data.
	Config map[string]any
}

// MaterializationResult records the outcome of writing files to disk.
type MaterializationResult struct {
	// WrittenFiles lists the absolute paths of files written.
	WrittenFiles []string
	// CreatedDirs lists the absolute paths of directories created.
	CreatedDirs []string
	// SymlinkedFiles lists the absolute paths of symlinks created.
	SymlinkedFiles []string
	// Errors lists any errors encountered during materialization.
	Errors []MaterializationError
}

// MaterializationError records a single materialization failure.
type MaterializationError struct {
	// Path is the target file path that failed.
	Path string
	// Err is the error message.
	Err string
}

// BuildReport is the final output of a compiler run. It contains provenance
// records, diagnostics, lowering records, and build metadata for all
// build units processed.
type BuildReport struct {
	// Timestamp is when the build completed.
	Timestamp time.Time
	// Duration is how long the build took.
	Duration time.Duration
	// Units lists reports for each build unit.
	Units []UnitReport
	// Diagnostics holds all diagnostics emitted during the build.
	Diagnostics []Diagnostic
}

// UnitReport holds the build report for a single build unit.
type UnitReport struct {
	// Coordinate identifies the build unit.
	Coordinate build.BuildCoordinate
	// EmittedFiles lists the files emitted for this unit.
	EmittedFiles []string
	// LoweringRecords documents every lowering decision.
	LoweringRecords []LoweringRecord
	// SelectedProviders maps capability IDs to the provider selected.
	SelectedProviders map[string]string
	// SkippedObjects lists object IDs that were skipped.
	SkippedObjects []string
	// Warnings lists non-fatal warnings for this unit.
	Warnings []string
}

// LoweringRecord documents a single lowering decision with full provenance.
type LoweringRecord struct {
	// ObjectID is the original canonical object ID.
	ObjectID string
	// FromKind is the object's kind before lowering.
	FromKind model.Kind
	// ToKind is the object's kind after lowering.
	ToKind model.Kind
	// ToPath is the output file path where the lowered object was emitted.
	ToPath string
	// Reason explains why lowering was performed.
	Reason string
	// Preservation is the object's preservation level.
	Preservation model.Preservation
	// Status is the outcome ("lowered", "skipped", "failed").
	Status string
}

// Diagnostic is a compiler diagnostic message (error, warning, info).
type Diagnostic struct {
	// Severity is the diagnostic level ("error", "warning", "info").
	Severity string
	// Code is an optional machine-readable diagnostic code.
	Code string
	// Message is the human-readable diagnostic message.
	Message string
	// SourcePath is the source file that caused the diagnostic.
	SourcePath string
	// ObjectID is the canonical object ID related to this diagnostic.
	ObjectID string
	// Phase is the pipeline phase that emitted this diagnostic.
	Phase Phase
}
