package packaging

import (
	"context"

	"github.com/mariotoffia/goagentmeta/internal/domain/build"
)

func (s *PackagingService) packageOCI(
	_ context.Context,
	_ *OCIPackagingConfig,
	_ map[build.Target][]string,
) (*PackagedArtifact, error) {
	return nil, ErrNotImplemented
}
