package composer

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/fs"
	"github.com/paketo-buildpacks/packit/v2/scribe"
)

// PhpVersionResolverInterface provides a Resolve method to determine which Php Version to request
//go:generate faux --interface PhpVersionResolverInterface --output fakes/php_version_resolver.go
type PhpVersionResolverInterface interface {
	Resolve(composerJsonPath, composerLockPath string) (version, versionSource string, err error)
}

func Detect(logEmitter scribe.Emitter, phpVersionResolver PhpVersionResolverInterface) packit.DetectFunc {
	return func(context packit.DetectContext) (packit.DetectResult, error) {
		composerJsonPath, composerLockPath, composerVar, composerVarFound := FindComposerFiles(context.WorkingDir)

		if exists, err := fs.Exists(composerJsonPath); err != nil {
			return packit.DetectResult{}, err
		} else if !exists && !composerVarFound {
			return packit.DetectResult{}, packit.Fail.WithMessage("no %s found", DefaultComposerJsonPath)
		} else if !exists && composerVarFound {
			return packit.DetectResult{}, packit.Fail.WithMessage("no %s found at location '%s'", DefaultComposerJsonPath, composerVar)
		}

		if exists, err := fs.Exists(composerLockPath); err != nil {
			return packit.DetectResult{}, err
		} else if !exists {
			logEmitter.Title("WARNING: Include a 'composer.lock' file with your application! This will make sure the exact same version of dependencies are used when you build. It will also enable caching of your dependency layer.")
		}

		if composerVendorDir, found := os.LookupEnv(ComposerVendorDir); found {
			if relativePath, err := filepath.Rel(context.WorkingDir, filepath.Join(context.WorkingDir, composerVendorDir)); err != nil {
				return packit.DetectResult{}, err
			} else if relativePath != composerVendorDir || strings.HasPrefix(relativePath, "..") {
				return packit.DetectResult{}, packit.Fail.WithMessage("COMPOSER_VENDOR_DIR must be a relative path underneath the project root")
			}
		}

		phpRequirement := packit.BuildPlanRequirement{
			Name: PhpDependency,
			Metadata: BuildPlanMetadata{
				Build: true,
			},
		}

		if phpVersion, phpVersionSource, err := phpVersionResolver.Resolve(composerJsonPath, composerLockPath); err != nil {
			return packit.DetectResult{}, err
		} else if phpVersion != "" {
			phpRequirement.Metadata = BuildPlanMetadata{
				Build:         true,
				Version:       phpVersion,
				VersionSource: phpVersionSource,
			}
		}

		return packit.DetectResult{
			Plan: packit.BuildPlan{
				Provides: []packit.BuildPlanProvision{
					{
						Name: ComposerPackagesDependency,
					},
				},
				Requires: []packit.BuildPlanRequirement{
					{
						Name: ComposerDependency,
						Metadata: BuildPlanMetadata{
							Build: true,
						},
					},
					phpRequirement,
				},
			},
		}, nil
	}
}
