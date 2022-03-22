package composer

import (
	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/fs"
	"os"
	"path/filepath"
)

func Detect() packit.DetectFunc {
	return func(context packit.DetectContext) (packit.DetectResult, error) {
		composerPath, composerFound := os.LookupEnv("COMPOSER")
		if !composerFound {
			composerPath = "composer.json"
		}

		exists, err := fs.Exists(filepath.Join(context.WorkingDir, composerPath))
		if err != nil {
			return packit.DetectResult{}, err
		}

		if !exists && !composerFound {
			return packit.DetectResult{}, packit.Fail.WithMessage("no composer.json found")
		}

		if !exists && composerFound {
			return packit.DetectResult{}, packit.Fail.WithMessage("no composer.json found at location '%s'", composerPath)
		}

		var requirements []packit.BuildPlanRequirement

		if version, ok := os.LookupEnv("BP_COMPOSER_VERSION"); ok {
			requirements = append(requirements, packit.BuildPlanRequirement{
				Name: "composer",
				Metadata: BuildPlanMetadata{
					VersionSource: "BP_COMPOSER_VERSION",
					Version:       version,
				},
			})
		}

		return packit.DetectResult{
			Plan: packit.BuildPlan{
				Provides: []packit.BuildPlanProvision{
					{
						Name: "composer",
					},
				},
				Requires: requirements,
			},
		}, nil
	}
}
