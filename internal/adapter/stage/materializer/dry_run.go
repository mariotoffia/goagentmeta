package materializer

import (
	"path/filepath"
	"sort"

	"github.com/mariotoffia/goagentmeta/internal/domain/pipeline"
)

// simulateMaterialization produces a MaterializationResult that describes
// what WOULD be written without performing any actual I/O. This is used
// for dry-run mode and CI preview workflows.
func simulateMaterialization(plan pipeline.EmissionPlan) pipeline.MaterializationResult {
	var result pipeline.MaterializationResult

	unitKeys := make([]string, 0, len(plan.Units))
	for k := range plan.Units {
		unitKeys = append(unitKeys, k)
	}
	sort.Strings(unitKeys)

	for _, outputDir := range unitKeys {
		unit := plan.Units[outputDir]

		result.CreatedDirs = append(result.CreatedDirs, outputDir)

		for _, dir := range unit.Directories {
			result.CreatedDirs = append(result.CreatedDirs, filepath.Join(outputDir, dir))
		}

		for _, f := range unit.Files {
			result.WrittenFiles = append(result.WrittenFiles, filepath.Join(outputDir, f.Path))
		}

		for _, a := range unit.Assets {
			result.SymlinkedFiles = append(result.SymlinkedFiles, filepath.Join(outputDir, a.DestPath))
		}

		for _, s := range unit.Scripts {
			result.SymlinkedFiles = append(result.SymlinkedFiles, filepath.Join(outputDir, s.DestPath))
		}

		for _, pb := range unit.PluginBundles {
			bundleDir := filepath.Join(outputDir, pb.DestDir)
			result.CreatedDirs = append(result.CreatedDirs, bundleDir)
			for _, f := range pb.Files {
				result.WrittenFiles = append(result.WrittenFiles, filepath.Join(bundleDir, f.Path))
			}
		}
	}

	return result
}
