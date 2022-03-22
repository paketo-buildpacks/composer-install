package composer

import (
	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/chronos"
	"github.com/paketo-buildpacks/packit/v2/postal"
	"github.com/paketo-buildpacks/packit/v2/scribe"
	"os"
	"path/filepath"
	"time"
)

// Note that Go 1.18 requires faux 0.21.0 (https://github.com/ryanmoran/faux/releases/tag/v0.21.0)

//go:generate faux --interface DependencyManager --output fakes/dependency_manager.go
type DependencyManager interface {
	Resolve(path, id, version, stack string) (postal.Dependency, error)
	Deliver(dependency postal.Dependency, cnbPath, layerPath, platformPath string) error
}

//go:generate faux --interface EntryResolver --output fakes/entry_resolver.go
type EntryResolver interface {
	Resolve(name string, entries []packit.BuildpackPlanEntry, priorites []interface{}) (packit.BuildpackPlanEntry, []packit.BuildpackPlanEntry)
	MergeLayerTypes(string, []packit.BuildpackPlanEntry) (launch, build bool)
}

func Build(
	logger scribe.Emitter,
	dependencyManager DependencyManager,
	entryResolver EntryResolver) packit.BuildFunc {
	return func(context packit.BuildContext) (packit.BuildResult, error) {
		logger.Title("%s %s", context.BuildpackInfo.Name, context.BuildpackInfo.Version)
		logger.Process("Resolving Composer version")

		priorities := []interface{}{
			"BP_COMPOSER_VERSION",
		}
		entry, sortedEntries := entryResolver.Resolve("composer", context.Plan.Entries, priorities)
		logger.Candidates(sortedEntries)

		composerLayer, err := context.Layers.Get("composer")
		if err != nil {
			return packit.BuildResult{}, err
		}

		composerLayer, err = composerLayer.Reset()
		if err != nil {
			return packit.BuildResult{}, err
		}

		composerLayer.Launch, composerLayer.Build = entryResolver.MergeLayerTypes("composer", context.Plan.Entries)

		version, ok := entry.Metadata["version"].(string)
		if !ok {
			version = "default"
		}

		dependency, err := dependencyManager.Resolve(
			filepath.Join(context.CNBPath, "buildpack.toml"),
			entry.Name,
			version,
			context.Stack)
		if err != nil {
			return packit.BuildResult{}, err
		}

		clock := chronos.DefaultClock

		logger.SelectedDependency(entry, dependency, clock.Now())

		logger.Process("Executing build process")
		logger.Subprocess("Installing Composer %s", dependency.Version)

		duration, err := clock.Measure(func() error {
			return dependencyManager.Deliver(dependency, context.CNBPath, composerLayer.Path, context.Platform.Path)
		})
		if err != nil {
			return packit.BuildResult{}, err
		}
		logger.Action("Completed in %s", duration.Round(time.Millisecond))
		logger.Break()

		baseFilename := dependency.Name
		if baseFilename == "" {
			baseFilename = filepath.Base(dependency.URI)
		}

		logger.Debug.Subprocess("Delivered Composer filename %s", baseFilename)

		err = os.Chmod(filepath.Join(composerLayer.Path, baseFilename), 0755)
		if err != nil {
			return packit.BuildResult{}, err
		}

		err = os.MkdirAll(filepath.Join(composerLayer.Path, "bin"), os.ModePerm)
		if err != nil {
			return packit.BuildResult{}, err
		}

		symLink := filepath.Join(composerLayer.Path, "bin", "composer")
		logger.Debug.Subprocess("Creating Composer symlink at %s", baseFilename)
		err = os.Symlink(filepath.Join(composerLayer.Path, baseFilename), symLink)
		if err != nil {
			return packit.BuildResult{}, err
		}

		composerLayer.Metadata = map[string]interface{}{
			"dependency-sha": dependency.SHA256,
		}

		return packit.BuildResult{
			Layers: []packit.Layer{
				composerLayer,
			},
		}, nil
	}
}
