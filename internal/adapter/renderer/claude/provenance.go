package claude

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// injectProvenanceHeaders is intentionally a no-op. HTML comments in rendered
// markdown waste LLM context tokens without providing value to the AI reading
// them. Provenance is tracked in provenance.json instead.
func injectProvenanceHeaders(files []pipeline.EmittedFile) []pipeline.EmittedFile {
	return files
}

// renderProvenance generates a provenance.json file listing all source objects
// and output files for the build unit.
func renderProvenance(unitKey string, files []pipeline.EmittedFile) pipeline.EmittedFile {
	entries := make([]provenanceEntry, 0, len(files))

	// Collect all source→output mappings.
	sourceToOutputs := make(map[string][]string)
	for _, f := range files {
		for _, src := range f.SourceObjects {
			sourceToOutputs[src] = append(sourceToOutputs[src], f.Path)
		}
	}

	// Sort source keys for deterministic output.
	sourceKeys := make([]string, 0, len(sourceToOutputs))
	for k := range sourceToOutputs {
		sourceKeys = append(sourceKeys, k)
	}
	sort.Strings(sourceKeys)

	for _, src := range sourceKeys {
		outputs := sourceToOutputs[src]
		sort.Strings(outputs)
		entries = append(entries, provenanceEntry{
			SourceObject: src,
			OutputFiles:  outputs,
		})
	}

	prov := provenanceJSON{
		BuildUnit: unitKey,
		Target:    "claude",
		Entries:   entries,
	}

	data, err := json.MarshalIndent(prov, "", "  ")
	if err != nil {
		data = []byte("{}")
	}

	// Ensure trailing newline.
	data = append(data, '\n')

	var allSources []string
	for _, e := range entries {
		allSources = append(allSources, e.SourceObject)
	}

	return pipeline.EmittedFile{
		Path:          "provenance.json",
		Content:       data,
		Layer:         pipeline.LayerExtension,
		SourceObjects: allSources,
	}
}

// provenanceJSON is the structure of provenance.json.
type provenanceJSON struct {
	BuildUnit string            `json:"buildUnit"`
	Target    string            `json:"target"`
	Entries   []provenanceEntry `json:"entries"`
}

// provenanceEntry maps a source object to the output files it contributed to.
type provenanceEntry struct {
	SourceObject string   `json:"sourceObject"`
	OutputFiles  []string `json:"outputFiles"`
}

// isMarkdown checks if a file path ends with .md.
func isMarkdown(path string) bool {
	return strings.HasSuffix(path, ".md")
}
