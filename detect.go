package composer

import (
	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/fs"
	"github.com/paketo-buildpacks/packit/v2/scribe"
	"os"
	"path/filepath"
)

func Detect(logEmitter scribe.Emitter) packit.DetectFunc {
	return func(context packit.DetectContext) (packit.DetectResult, error) {
		composerPath, composerFound := os.LookupEnv("COMPOSER")
		if !composerFound {
			composerPath = "composer.json"
		}

		if exists, err := fs.Exists(filepath.Join(context.WorkingDir, composerPath)); err != nil {
			return packit.DetectResult{}, err
		} else if !exists && !composerFound {
			return packit.DetectResult{}, packit.Fail.WithMessage("no composer.json found")
		} else if !exists && composerFound {
			return packit.DetectResult{}, packit.Fail.WithMessage("no composer.json found at location '%s'", composerPath)
		}

		composerRequirement := packit.BuildPlanRequirement{
			Name: "composer",
			Metadata: BuildPlanMetadata{
				Build: true,
			},
		}

		if version, versionOk := os.LookupEnv("BP_COMPOSER_VERSION"); versionOk {
			composerRequirement.Metadata = BuildPlanMetadata{
				VersionSource: "BP_COMPOSER_VERSION",
				Version:       version,
				Build:         true,
			}
		}

		phpRequirement := packit.BuildPlanRequirement{
			Name: "php",
			Metadata: BuildPlanMetadata{
				Build: true,
			},
		}

		if version, versionOk := os.LookupEnv("BP_PHP_VERSION"); versionOk {
			phpRequirement.Metadata = BuildPlanMetadata{
				VersionSource: "BP_PHP_VERSION",
				Version:       version,
				Build:         true,
			}
		}

		return packit.DetectResult{
			Plan: packit.BuildPlan{
				Provides: []packit.BuildPlanProvision{},
				Requires: []packit.BuildPlanRequirement{
					composerRequirement,
					phpRequirement,
				},
			},
		}, nil
	}
}
