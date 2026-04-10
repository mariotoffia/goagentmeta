package model_test

import (
	"testing"

	"github.com/mariotoffia/goagentmeta/internal/domain/model"
)

func TestObjectMetaDescription(t *testing.T) {
	meta := model.ObjectMeta{
		ID:          "test-skill",
		Kind:        model.KindSkill,
		Description: "A reusable Go benchmarking skill.",
	}

	if meta.Description != "A reusable Go benchmarking skill." {
		t.Errorf("Description = %q, want %q", meta.Description, "A reusable Go benchmarking skill.")
	}
}

func TestObjectMetaPackageVersion(t *testing.T) {
	meta := model.ObjectMeta{
		ID:             "go-lambda",
		Kind:           model.KindSkill,
		Version:        1,
		PackageVersion: "1.1.3",
	}

	if meta.Version != 1 {
		t.Errorf("Version (schema) = %d, want 1", meta.Version)
	}
	if meta.PackageVersion != "1.1.3" {
		t.Errorf("PackageVersion = %q, want %q", meta.PackageVersion, "1.1.3")
	}
}

func TestObjectMetaLicense(t *testing.T) {
	meta := model.ObjectMeta{
		ID:      "go-lambda",
		Kind:    model.KindSkill,
		License: "MIT",
	}

	if meta.License != "MIT" {
		t.Errorf("License = %q, want %q", meta.License, "MIT")
	}
}

func TestObjectMetaVersionAndPackageVersionAreDistinct(t *testing.T) {
	meta := model.ObjectMeta{
		ID:             "test-obj",
		Kind:           model.KindPlugin,
		Version:        2,
		PackageVersion: "3.0.0-beta.1",
		License:        "Apache-2.0",
	}

	if meta.Version == 0 {
		t.Error("Version should be 2, not zero")
	}
	if meta.PackageVersion == "" {
		t.Error("PackageVersion should not be empty")
	}
	if meta.Version == 2 && meta.PackageVersion == "3.0.0-beta.1" {
		// Correct: schema version (int) ≠ package version (semver string)
		return
	}
	t.Errorf("Version=%d PackageVersion=%q mismatch", meta.Version, meta.PackageVersion)
}
