package plugin_test

import (
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/domain/model"
	"github.com/mariotoffia/goagentmeta/internal/domain/plugin"
)

// TestPluginDescriptionFromObjectMeta verifies that Plugin.Description is
// inherited from ObjectMeta, not a separate field.
func TestPluginDescriptionFromObjectMeta(t *testing.T) {
	p := plugin.Plugin{
		ObjectMeta: model.ObjectMeta{
			ID:          "repo-graph",
			Kind:        model.KindPlugin,
			Description: "Provides repository graph queries as a runtime extension",
			License:     "Apache-2.0",
		},
		Distribution: plugin.Distribution{
			Mode: plugin.DistInline,
		},
		Provides: []string{"repo.graph.query"},
	}

	if p.Description != "Provides repository graph queries as a runtime extension" {
		t.Errorf("Description = %q, want %q",
			p.Description, "Provides repository graph queries as a runtime extension")
	}
	if p.License != "Apache-2.0" {
		t.Errorf("License = %q, want %q", p.License, "Apache-2.0")
	}
}
