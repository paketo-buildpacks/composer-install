package composer

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/chronos"
	"github.com/paketo-buildpacks/packit/v2/draft"
	"github.com/paketo-buildpacks/packit/v2/fs"
	"github.com/paketo-buildpacks/packit/v2/pexec"
	"github.com/paketo-buildpacks/packit/v2/sbom"
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

//go:generate faux --interface SBOMGenerator --output fakes/sbom_generator.go
type SBOMGenerator interface {
	Generate(dir string) (sbom.SBOM, error)
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
	composerDumpAutoloadExec Executable,
	composerGlobalExec Executable,
	checkPlatformReqsExec Executable,
	sbomGenerator SBOMGenerator,
	path string,
	calculator Calculator,
	clock chronos.Clock) packit.BuildFunc {
	return func(context packit.BuildContext) (packit.BuildResult, error) {
		logger.Title("%s %s", context.BuildpackInfo.Name, context.BuildpackInfo.Version)

		composerPhpIniPath, err := writeComposerPhpIni(logger, context)
		if err != nil { // untested
			return packit.BuildResult{}, err
		}

		composerGlobalBin, err := runComposerGlobalIfRequired(logger, context, composerGlobalExec, path, composerPhpIniPath)
		if err != nil { // untested
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

		var composerPackagesLayer packit.Layer
		logger.Process("Executing build process")
		duration, err := clock.Measure(func() error {
			composerPackagesLayer, err = runComposerInstall(
				logger,
				context,
				composerInstallOptions,
				composerPhpIniPath,
				path,
				composerInstallExec,
				composerDumpAutoloadExec,
				workspaceVendorDir,
				calculator)
			return err
		})
		if err != nil {
			return packit.BuildResult{}, err
		}
		logger.Action("Completed in %s", duration.Round(time.Millisecond))
		logger.Break()

		logger.GeneratingSBOM(composerPackagesLayer.Path)

		var sbomContent sbom.SBOM
		duration, err = clock.Measure(func() error {
			sbomContent, err = sbomGenerator.Generate(context.WorkingDir)
			return err
		})
		if err != nil {
			return packit.BuildResult{}, err
		}
		logger.Action("Completed in %s", duration.Round(time.Millisecond))
		logger.Break()

		logger.FormattingSBOM(context.BuildpackInfo.SBOMFormats...)

		composerPackagesLayer.SBOM, err = sbomContent.InFormats(context.BuildpackInfo.SBOMFormats...)
		if err != nil {
			return packit.BuildResult{}, err
		}

		err = runCheckPlatformReqs(logger, checkPlatformReqsExec, context.WorkingDir, composerPhpIniPath, path)
		if err != nil {
			return packit.BuildResult{}, err
		}

		return packit.BuildResult{
			Layers: []packit.Layer{
				composerPackagesLayer,
			},
		}, nil
	}
}

// runComposerGlobalIfRequired will check for existence of env var "BP_COMPOSER_INSTALL_GLOBAL".
// If that exists, will run `composer global require` with the contents of BP_COMPOSER_INSTALL_GLOBAL
// to ensure that those packages are available for Composer scripts.
//
// It will return the location to which the packages have been installed, so that they can be made available
// on the path when running `composer install`.
//
// `composer global require`: https://getcomposer.org/doc/03-cli.md#global
// Composer scripts: https://getcomposer.org/doc/articles/scripts.md
func runComposerGlobalIfRequired(
	logger scribe.Emitter,
	context packit.BuildContext,
	composerGlobalExec Executable,
	path string,
	composerPhpIniPath string) (composerGlobalBin string, err error) {
	composerInstallGlobal, found := os.LookupEnv(BpComposerInstallGlobal)

	if !found {
		return "", nil
	}

	logger.Process("Running 'composer global require'")

	composerGlobalLayer, err := context.Layers.Get(ComposerGlobalLayerName)
	if err != nil { // untested
		return "", err
	}

	composerGlobalLayer, err = composerGlobalLayer.Reset()
	if err != nil { // untested
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

	logger.Debug.Subprocess(composerGlobalBuffer.String())
	logger.Process("Ran 'composer %s'", strings.Join(execution.Args, " "))

	composerGlobalBin = filepath.Join(composerGlobalLayer.Path, "vendor", "bin")

	if os.Getenv(BpLogLevel) == "DEBUG" {
		logger.Debug.Subprocess("Adding global Composer packages to PATH:")
		files, err := os.ReadDir(composerGlobalBin)
		if err != nil { // untested
			return "", err
		}
		for _, f := range files {
			logger.Debug.Subprocess(fmt.Sprintf("- %s", f.Name()))
		}
	}

	return
}

// runComposerInstall will run `composer install` to download dependencies into a new layer
//
// Returns:
// - composerPackagesLayer: a new layer into which the dependencies will be installed
// - layerVendorDir: the absolute file path inside the layer where the dependencies are installed
// - err: any error
//
// https://getcomposer.org/doc/03-cli.md#install-i
func runComposerInstall(
	logger scribe.Emitter,
	context packit.BuildContext,
	composerInstallOptions DetermineComposerInstallOptions,
	composerPhpIniPath string,
	path string,
	composerInstallExec Executable,
	composerDumpAutoloadExec Executable,
	workspaceVendorDir string,
	calculator Calculator) (composerPackagesLayer packit.Layer, err error) {

	launch, build := draft.NewPlanner().MergeLayerTypes(ComposerPackagesDependency, context.Plan.Entries)

	composerPackagesLayer, err = context.Layers.Get(ComposerPackagesLayerName)
	if err != nil { // untested
		return packit.Layer{}, err
	}

	composerJsonPath, composerLockPath, _, _ := FindComposerFiles(context.WorkingDir)

	layerVendorDir := filepath.Join(composerPackagesLayer.Path, "vendor")

	composerLockChecksum, err := calculator.Sum(composerLockPath)
	if err != nil { // untested
		return packit.Layer{}, err
	}

	logger.Debug.Process("Calculated checksum of %s for composer.lock", composerLockChecksum)

	stack, stackOk := composerPackagesLayer.Metadata["stack"]
	if stackOk {
		logger.Debug.Process("Previous stack: %s", stack.(string))
		logger.Debug.Process("Current stack: %s", context.Stack)
	}

	cachedSHA, shaOk := composerPackagesLayer.Metadata["composer-lock-sha"].(string)
	if (shaOk && cachedSHA == composerLockChecksum) && (stackOk && stack.(string) == context.Stack) {
		logger.Process("Reusing cached layer %s", composerPackagesLayer.Path)
		logger.Break()

		composerPackagesLayer.Launch, composerPackagesLayer.Build, composerPackagesLayer.Cache = launch, build, build

		logger.Debug.Subprocess("Setting cached layer types: launch=[%t], build=[%t], cache=[%t]",
			composerPackagesLayer.Launch,
			composerPackagesLayer.Build,
			composerPackagesLayer.Cache)

		logger.Process("Writing symlink %s => %s", workspaceVendorDir, layerVendorDir)
		if os.Getenv(BpLogLevel) == "DEBUG" {
			logger.Debug.Subprocess("Listing files in %s:", layerVendorDir)
			files, err := os.ReadDir(layerVendorDir)
			if err != nil { // untested
				return packit.Layer{}, err
			}
			for _, f := range files {
				logger.Debug.Subprocess(fmt.Sprintf("- %s", f.Name()))
			}
		}

		err = os.Symlink(layerVendorDir, workspaceVendorDir)
		if err != nil { // untested
			return packit.Layer{}, err
		}

		return composerPackagesLayer, nil
	}

	logger.Process("Building new layer %s", composerPackagesLayer.Path)

	composerPackagesLayer, err = composerPackagesLayer.Reset()
	if err != nil { // untested
		return packit.Layer{}, err
	}

	composerPackagesLayer.Launch, composerPackagesLayer.Build, composerPackagesLayer.Cache = launch, build, build

	logger.Debug.Subprocess("Setting layer types: launch=[%t], build=[%t], cache=[%t]",
		composerPackagesLayer.Launch,
		composerPackagesLayer.Build,
		composerPackagesLayer.Cache)

	composerPackagesLayer.Metadata = map[string]interface{}{
		"stack":             context.Stack,
		"composer-lock-sha": composerLockChecksum,
	}

	if exists, err := fs.Exists(workspaceVendorDir); err != nil {
		return packit.Layer{}, err
	} else if exists {
		logger.Process("Detected existing vendored packages, will run 'composer install' with those packages")
		if err := fs.Copy(workspaceVendorDir, layerVendorDir); err != nil { // untested
			return packit.Layer{}, err
		}
		if err := os.RemoveAll(workspaceVendorDir); err != nil { // untested
			return packit.Layer{}, err
		}
	}

	composerInstallBuffer := bytes.NewBuffer(nil)

	// `composer install` will run with `--no-autoloader` to avoid errors from
	// autoloading classes outside of the vendor directory

	// Once `composer install` has run, the symlink to the working directory is
	// set up, and then `composer dump-autoload` on the vendor directory from
	// the working directory.
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
		return packit.Layer{}, err
	}

	logger.Debug.Subprocess(composerInstallBuffer.String())
	logger.Process("Ran 'composer %s'", strings.Join(execution.Args, " "))

	logger.Process("Writing symlink %s => %s", workspaceVendorDir, layerVendorDir)
	if os.Getenv(BpLogLevel) == "DEBUG" {
		logger.Debug.Subprocess("Listing files in %s:", layerVendorDir)
		files, err := os.ReadDir(layerVendorDir)
		if err != nil { // untested
			return packit.Layer{}, err
		}
		for _, f := range files {
			logger.Debug.Subprocess(fmt.Sprintf("- %s", f.Name()))
		}
	}

	err = os.Symlink(layerVendorDir, workspaceVendorDir)
	if err != nil { // untested
		return packit.Layer{}, err
	}

	logger.Process("Running 'composer dump-autoload'")

	composerAutoloadBuffer := bytes.NewBuffer(nil)
	execution = pexec.Execution{
		Args: append([]string{"dump-autoload", "--classmap-authoritative"}),
		Dir:  context.WorkingDir,
		Env: append(os.Environ(),
			"COMPOSER_NO_INTERACTION=1", // https://getcomposer.org/doc/03-cli.md#composer-no-interaction
			fmt.Sprintf("COMPOSER_VENDOR_DIR=%s", workspaceVendorDir),
			fmt.Sprintf("PHPRC=%s", composerPhpIniPath),
			fmt.Sprintf("PATH=%s", path),
		),
		Stdout: composerAutoloadBuffer,
		Stderr: composerAutoloadBuffer,
	}

	err = composerDumpAutoloadExec.Execute(execution)
	if err != nil {
		logger.Subprocess(composerAutoloadBuffer.String())
		return packit.Layer{}, err
	}

	logger.Debug.Subprocess(composerAutoloadBuffer.String())
	logger.Process("Ran 'composer %s'", strings.Join(execution.Args, " "))

	return composerPackagesLayer, nil
}

// writeComposerPhpIni will create a PHP INI file used by Composer itself,
// such as when running `composer global` and `composer install.
// This is created in a new ignored layer.
func writeComposerPhpIni(logger scribe.Emitter, context packit.BuildContext) (composerPhpIniPath string, err error) {
	composerPhpIniLayer, err := context.Layers.Get(ComposerPhpIniLayerName)
	if err != nil { // untested
		return "", err
	}

	composerPhpIniLayer, err = composerPhpIniLayer.Reset()
	if err != nil { // untested
		return "", err
	}

	composerPhpIniPath = filepath.Join(composerPhpIniLayer.Path, "composer-php.ini")

	logger.Debug.Process("Writing php.ini for composer")
	logger.Debug.Subprocess("Writing %s to %s", filepath.Base(composerPhpIniPath), composerPhpIniPath)

	phpIni := fmt.Sprintf(`[PHP]
extension_dir = "%s"
extension = openssl.so`, os.Getenv(PhpExtensionDir))
	logger.Debug.Subprocess("Writing php.ini contents:\n'%s'", phpIni)

	return composerPhpIniPath, os.WriteFile(composerPhpIniPath, []byte(phpIni), os.ModePerm)
}

// runCheckPlatformReqs will run Composer command `check-platform-reqs`
// to see which platform requirements are "missing".
// https://getcomposer.org/doc/03-cli.md#check-platform-reqs
//
// Any "missing" requirements will be added to an INI file that should be autoloaded via PHP_INI_SCAN_DIR,
// when used in conjunction with the `php-dist` Paketo Buildpack
// INI file location: {workingDir}/.php.ini.d/composer-extensions.ini
// PHP_INI_SCAN_DIR: https://github.com/paketo-buildpacks/php-dist/blob/bfed65e9c3b59cf2c5aee3752d82470f8259f655/build.go#L219-L223
// Requires `php-dist` 0.8.0+ (https://github.com/paketo-buildpacks/php-dist/releases/tag/v0.8.0)
//
// This code has been largely borrowed from the original `php-composer` buildpack
// https://github.com/paketo-buildpacks/php-composer/blob/5e2604b74cbeb30090bf7eadb1cfc158b374efc0/composer/composer.go#L76-L100
//
// In case you are curious about exit code 2: https://getcomposer.org/doc/03-cli.md#process-exit-codes
func runCheckPlatformReqs(logger scribe.Emitter, checkPlatformReqsExec Executable, workingDir, composerPhpIniPath, path string) error {
	buffer := bytes.NewBuffer(nil)

	logger.Process("Running 'composer check-platform-reqs'")

	execution := pexec.Execution{
		Args: []string{"check-platform-reqs"},
		Dir:  workingDir,
		Env: append(os.Environ(),
			"COMPOSER_NO_INTERACTION=1", // https://getcomposer.org/doc/03-cli.md#composer-no-interaction
			fmt.Sprintf("PHPRC=%s", composerPhpIniPath),
			fmt.Sprintf("PATH=%s", path),
		),
		Stdout: buffer,
		Stderr: buffer,
	}
	err := checkPlatformReqsExec.Execute(execution)
	if err != nil {
		logger.Subprocess(buffer.String())
		exitError, ok := err.(*exec.ExitError)
		if !ok || exitError.ExitCode() != 2 {
			return err
		}
	}

	var extensions []string
	for _, line := range strings.Split(buffer.String(), "\n") {
		chunks := strings.Split(strings.TrimSpace(line), " ")
		extensionName := strings.TrimPrefix(strings.TrimSpace(chunks[0]), "ext-")
		extensionStatus := strings.TrimSpace(chunks[len(chunks)-1])
		if extensionName != "php" && extensionName != "php-64bit" && extensionStatus == "missing" {
			extensions = append(extensions, extensionName)
		}
	}

	logger.Process("Ran 'composer check-platform-reqs', found extensions '%s'", strings.Join(extensions, ", "))

	buf := bytes.Buffer{}

	for _, extension := range extensions {
		buf.WriteString(fmt.Sprintf("extension = %s.so\n", extension))
	}

	iniDir := filepath.Join(workingDir, ".php.ini.d")

	err = os.MkdirAll(iniDir, os.ModeDir|os.ModePerm)
	if err != nil { // untested
		return err
	}

	return os.WriteFile(filepath.Join(iniDir, "composer-extensions.ini"), buf.Bytes(), 0666)
}
