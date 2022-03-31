package composer

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/draft"
	"github.com/paketo-buildpacks/packit/v2/fs"
	"github.com/paketo-buildpacks/packit/v2/pexec"
	"github.com/paketo-buildpacks/packit/v2/scribe"
)

// DetermineComposerInstallOptions defines the interface to get options for `composer install`
//go:generate faux --interface DetermineComposerInstallOptions --output fakes/determine_composer_install_options.go
type DetermineComposerInstallOptions interface {
	Determine() []string
}

// Executable just provides a fake for pexec.Executable for testing
//go:generate faux --interface Executable --output fakes/executable.go
type Executable interface {
	Execute(pexec.Execution) (err error)
}

// Calculator defines the interface for calculating a checksum of the given set
// of file paths.
//go:generate faux --interface Calculator --output fakes/calculator.go
type Calculator interface {
	Sum(paths ...string) (string, error)
}

func Build(
	logger scribe.Emitter,
	composerInstallOptions DetermineComposerInstallOptions,
	composerInstallExec Executable,
	composerGlobalExec Executable,
	path string,
	calculator Calculator) packit.BuildFunc {
	return func(context packit.BuildContext) (packit.BuildResult, error) {
		logger.Title("%s %s", context.BuildpackInfo.Name, context.BuildpackInfo.Version)

		composerPhpIniPath, err := writeComposerPhpIni(logger, context)
		if err != nil {
			return packit.BuildResult{}, err
		}

		composerGlobalBin, err := runComposerGlobalIfRequired(logger, context, composerGlobalExec, path, composerPhpIniPath)
		if err != nil {
			return packit.BuildResult{}, err
		}

		if composerGlobalBin != "" {
			path = strings.Join([]string{
				composerGlobalBin,
				path,
			}, string(os.PathListSeparator))
		}

		workspaceVendorDir := filepath.Join(context.WorkingDir, "vendor")

		if value, found := os.LookupEnv(ComposerVendorDir); found {
			workspaceVendorDir = filepath.Join(context.WorkingDir, value)
		}

		composerPackagesLayer, layerVendorDir, err := runComposerInstall(
			logger,
			context,
			composerInstallOptions,
			composerPhpIniPath,
			path,
			composerInstallExec,
			workspaceVendorDir,
			calculator)
		if err != nil {
			return packit.BuildResult{}, err
		}

		logger.Process("Writing symlink to %s", workspaceVendorDir)

		err = os.Symlink(layerVendorDir, workspaceVendorDir)
		if err != nil {
			return packit.BuildResult{}, err
		}

		if os.Getenv(BpLogLevel) == "DEBUG" {
			logger.Debug.Action("listing files in %s:", layerVendorDir)
			files, err := ioutil.ReadDir(layerVendorDir)
			if err != nil {
				return packit.BuildResult{}, err
			}
			for _, f := range files {
				logger.Debug.Detail(fmt.Sprintf("- %s", f.Name()))
			}
		}

		return packit.BuildResult{
			Layers: []packit.Layer{
				composerPackagesLayer,
			},
		}, nil
	}
}

func runComposerGlobalIfRequired(
	logger scribe.Emitter,
	context packit.BuildContext,
	composerGlobalExec Executable,
	path string,
	composerPhpIniPath string) (string, error) {
	composerInstallGlobal, found := os.LookupEnv(BpComposerInstallGlobal)

	if !found {
		return "", nil
	}

	logger.Process("Running 'composer global require'")

	composerGlobalLayer, err := context.Layers.Get(ComposerGlobalLayerName)
	if err != nil {
		return "", err
	}

	composerGlobalLayer, err = composerGlobalLayer.Reset()
	if err != nil {
		return "", err
	}

	globalPackages := strings.Split(composerInstallGlobal, " ")

	composerGlobalBuffer := bytes.NewBuffer(nil)

	execution := pexec.Execution{
		Args: append([]string{"global", "require", "--no-progress"}, globalPackages...),
		Dir:  composerGlobalLayer.Path,
		Env: append(os.Environ(),
			"COMPOSER_NO_INTERACTION=1", // https://getcomposer.org/doc/03-cli.md#composer-no-interaction
			fmt.Sprintf("COMPOSER_HOME=%s", composerGlobalLayer.Path),
			fmt.Sprintf("PHPRC=%s", composerPhpIniPath),
			"COMPOSER_VENDOR_DIR=vendor", // ensure default in the layer
			fmt.Sprintf("PATH=%s", path),
		),
		Stdout: composerGlobalBuffer,
		Stderr: composerGlobalBuffer,
	}
	err = composerGlobalExec.Execute(execution)

	if err != nil {
		logger.Subprocess(composerGlobalBuffer.String())
		return "", err
	}

	composerGlobalBin := filepath.Join(composerGlobalLayer.Path, "vendor", "bin")

	if os.Getenv(BpLogLevel) == "DEBUG" {
		logger.Debug.Subprocess(composerGlobalBuffer.String())
		logger.Debug.Action("Adding global Composer packages to PATH:")
		files, err := ioutil.ReadDir(composerGlobalBin)
		if err != nil {
			return "", err
		}
		for _, f := range files {
			logger.Debug.Detail(fmt.Sprintf("- %s", f.Name()))
		}
	}

	return composerGlobalBin, nil
}

func runComposerInstall(
	logger scribe.Emitter,
	context packit.BuildContext,
	composerInstallOptions DetermineComposerInstallOptions,
	composerPhpIniPath string,
	path string,
	composerInstallExec Executable,
	workspaceVendorDir string,
	calculator Calculator) (packit.Layer, string, error) {

	launch, build := draft.NewPlanner().MergeLayerTypes(ComposerPackagesDependency, context.Plan.Entries)

	composerPackagesLayer, err := context.Layers.Get(ComposerPackagesLayerName)
	if err != nil {
		return packit.Layer{}, "", err
	}

	composerJsonPath, composerLockPath, _, _ := FindComposerFiles(context.WorkingDir)

	layerVendorDir := filepath.Join(composerPackagesLayer.Path, "vendor")

	composerLockChecksum, err := calculator.Sum(composerLockPath)
	if err != nil {
		return packit.Layer{}, "", err
	}

	logger.Debug.Subprocess("Calculated checksum of %s for composer.lock", composerLockChecksum)

	if cachedSHA, ok := composerPackagesLayer.Metadata["composer-lock-sha"].(string); ok && cachedSHA == composerLockChecksum {
		logger.Process("Reusing cached layer %s", composerPackagesLayer.Path)
		logger.Break()

		composerPackagesLayer.Launch, composerPackagesLayer.Build, composerPackagesLayer.Cache = launch, build, launch

		logger.Debug.Subprocess("Setting cached layer types: launch=[%t], build=[%t], cache=[%t]",
			composerPackagesLayer.Launch,
			composerPackagesLayer.Build,
			composerPackagesLayer.Cache)

		return composerPackagesLayer, layerVendorDir, nil
	}

	composerPackagesLayer, err = composerPackagesLayer.Reset()
	if err != nil {
		return packit.Layer{}, "", err
	}

	composerPackagesLayer.Launch, composerPackagesLayer.Build, composerPackagesLayer.Cache = launch, build, launch

	logger.Debug.Subprocess("Setting layer types: launch=[%t], build=[%t], cache=[%t]",
		composerPackagesLayer.Launch,
		composerPackagesLayer.Build,
		composerPackagesLayer.Cache)

	composerPackagesLayer.Metadata = map[string]interface{}{
		"composer-lock-sha": composerLockChecksum,
	}

	if exists, err := fs.Exists(workspaceVendorDir); err != nil {
		return packit.Layer{}, "", err
	} else if exists {
		if err := fs.Copy(workspaceVendorDir, layerVendorDir); err != nil {
			return packit.Layer{}, "", err
		}
		if err := os.RemoveAll(workspaceVendorDir); err != nil {
			return packit.Layer{}, "", err
		}
	}

	composerInstallBuffer := bytes.NewBuffer(nil)

	logger.Process("Running 'composer install'")

	execution := pexec.Execution{
		Args: append([]string{"install"}, composerInstallOptions.Determine()...),
		Dir:  composerPackagesLayer.Path,
		Env: append(os.Environ(),
			"COMPOSER_NO_INTERACTION=1", // https://getcomposer.org/doc/03-cli.md#composer-no-interaction
			fmt.Sprintf("COMPOSER=%s", composerJsonPath),
			fmt.Sprintf("COMPOSER_HOME=%s", filepath.Join(composerPackagesLayer.Path, ".composer")),
			"COMPOSER_VENDOR_DIR=vendor", // ensure default in the layer
			fmt.Sprintf("PHPRC=%s", composerPhpIniPath),
			fmt.Sprintf("PATH=%s", path),
		),
		Stdout: composerInstallBuffer,
		Stderr: composerInstallBuffer,
	}
	err = composerInstallExec.Execute(execution)

	if err != nil {
		logger.Subprocess(composerInstallBuffer.String())
		return packit.Layer{}, "", err
	}

	logger.Debug.Subprocess(composerInstallBuffer.String())

	return composerPackagesLayer, layerVendorDir, nil
}

func writeComposerPhpIni(logger scribe.Emitter, context packit.BuildContext) (string, error) {
	composerPhpIniLayer, err := context.Layers.Get(ComposerPhpIniLayerName)
	if err != nil {
		return "", err
	}

	composerPhpIniLayer, err = composerPhpIniLayer.Reset()
	if err != nil {
		return "", err
	}

	composerPhpIniPath := filepath.Join(composerPhpIniLayer.Path, "composer-php.ini")

	logger.Debug.Process("Writing php.ini for composer")
	logger.Debug.Subprocess("Writing %s to %s", filepath.Base(composerPhpIniPath), composerPhpIniPath)

	phpIni := fmt.Sprintf(`[PHP]
extension_dir = "%s"
extension = openssl.so`, os.Getenv(PhpExtensionDir))
	logger.Debug.Subprocess("Writing php.ini contents: '%s'", phpIni)

	return composerPhpIniPath, os.WriteFile(composerPhpIniPath, []byte(phpIni), os.ModePerm)
}
