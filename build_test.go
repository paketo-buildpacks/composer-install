package composer_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/paketo-buildpacks/composer"
	"github.com/paketo-buildpacks/composer/fakes"
	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/chronos"
	"github.com/paketo-buildpacks/packit/v2/pexec"
	"github.com/paketo-buildpacks/packit/v2/sbom"
	"github.com/paketo-buildpacks/packit/v2/scribe"
	"github.com/sclevine/spec"
)

func testBuild(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		buffer                                  *bytes.Buffer
		installOptions                          *fakes.DetermineComposerInstallOptions
		composerInstallExecutable               *fakes.Executable
		composerGlobalExecutable                *fakes.Executable
		composerCheckPlatformReqsExecExecutable *fakes.Executable
		composerInstallExecution                pexec.Execution
		composerGlobalExecution                 pexec.Execution
		composerCheckPlatformReqsExecExecution  pexec.Execution
		sbomGenerator                           *fakes.SBOMGenerator
		calculator                              *fakes.Calculator

		layersDir  string
		workingDir string

		buildpackPlan packit.BuildpackPlan
		buildpackInfo packit.BuildpackInfo

		build packit.BuildFunc
	)

	it.Before(func() {
		var err error
		layersDir, err = os.MkdirTemp("", "layers")
		Expect(err).NotTo(HaveOccurred())

		workingDir, err = os.MkdirTemp("", "working-dir")
		Expect(err).NotTo(HaveOccurred())

		buffer = bytes.NewBuffer(nil)
		installOptions = &fakes.DetermineComposerInstallOptions{}
		composerInstallExecutable = &fakes.Executable{}
		composerGlobalExecutable = &fakes.Executable{}
		composerCheckPlatformReqsExecExecutable = &fakes.Executable{}

		composerInstallExecutable.ExecuteCall.Stub = func(temp pexec.Execution) error {
			Expect(os.MkdirAll(filepath.Join(layersDir, composer.ComposerPackagesLayerName, "vendor", "local-package-name"), os.ModeDir|os.ModePerm)).To(Succeed())
			Expect(fmt.Fprint(temp.Stdout, "stdout from composer install\n")).To(Equal(29))
			Expect(fmt.Fprint(temp.Stderr, "stderr from composer install\n")).To(Equal(29))
			composerInstallExecution = temp
			return nil
		}

		composerGlobalExecutable.ExecuteCall.Stub = func(temp pexec.Execution) error {
			Expect(os.MkdirAll(filepath.Join(layersDir, composer.ComposerGlobalLayerName, "vendor", "bin", "global-package-name"), os.ModeDir|os.ModePerm)).To(Succeed())
			Expect(fmt.Fprint(temp.Stdout, "stdout from composer global\n")).To(Equal(28))
			Expect(fmt.Fprint(temp.Stderr, "stderr from composer global\n")).To(Equal(28))
			composerGlobalExecution = temp
			return nil
		}

		composerCheckPlatformReqsExecExecutable.ExecuteCall.Stub = func(temp pexec.Execution) error {
			composerCheckPlatformReqsExecExecution = temp

			_, err := temp.Stdout.Write([]byte(`ext-hello  8.1.4    missing
ext-foo   8.1.4    success
ext-bar   8.1.4    missing
php       8.1.4    success
`))

			Expect(err).NotTo(HaveOccurred())

			return nil
		}

		sbomGenerator = &fakes.SBOMGenerator{}
		sbomGenerator.GenerateCall.Returns.SBOM = sbom.SBOM{}
		calculator = &fakes.Calculator{}
		calculator.SumCall.Returns.String = "default-checksum"

		Expect(os.Setenv("PHP_EXTENSION_DIR", "php-extension-dir"))

		installOptions.DetermineCall.Returns.StringSlice = []string{
			"options",
			"from",
			"fake",
		}

		build = composer.Build(
			scribe.NewEmitter(buffer).WithLevel("DEBUG"),
			installOptions,
			composerInstallExecutable,
			composerGlobalExecutable,
			composerCheckPlatformReqsExecExecutable,
			sbomGenerator,
			"fake-path-from-tests",
			calculator,
			chronos.DefaultClock)

		buildpackInfo = packit.BuildpackInfo{
			Name:        "Some Buildpack",
			Version:     "some-version",
			SBOMFormats: []string{sbom.CycloneDXFormat, sbom.SPDXFormat},
		}

		buildpackPlan = packit.BuildpackPlan{
			Entries: []packit.BuildpackPlanEntry{
				{
					Name: composer.ComposerPackagesDependency,
					Metadata: map[string]interface{}{
						"build":  false,
						"launch": true,
					},
				},
			},
		}
	})

	it.After(func() {
		Expect(os.RemoveAll(layersDir)).To(Succeed())
		Expect(os.RemoveAll(workingDir)).To(Succeed())
		Expect(os.Unsetenv("COMPOSER")).To(Succeed())
		Expect(os.Unsetenv("PHP_EXTENSION_DIR")).To(Succeed())
	})

	context("without COMPOSER set", func() {
		it("contributes a layer called 'composer-packages'", func() {
			result, err := build(
				packit.BuildContext{
					BuildpackInfo: buildpackInfo,
					WorkingDir:    workingDir,
					Layers:        packit.Layers{Path: layersDir},
					Plan:          buildpackPlan,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			layers := result.Layers
			Expect(layers).To(HaveLen(1))

			packagesLayer := layers[0]
			Expect(packagesLayer.Name).To(Equal(composer.ComposerPackagesLayerName))
			Expect(packagesLayer.Path).To(Equal(filepath.Join(layersDir, composer.ComposerPackagesLayerName)))

			Expect(packagesLayer.Build).To(BeFalse())
			Expect(packagesLayer.Launch).To(BeTrue())
			Expect(packagesLayer.Cache).To(BeFalse())

			Expect(packagesLayer.BuildEnv).To(BeEmpty())
			Expect(packagesLayer.LaunchEnv).To(BeEmpty())
			Expect(packagesLayer.ProcessLaunchEnv).To(BeEmpty())
			Expect(packagesLayer.Metadata["composer-lock-sha"]).To(Equal("default-checksum"))

			Expect(packagesLayer.SBOM.Formats()).To(Equal([]packit.SBOMFormat{
				{
					Extension: sbom.Format(sbom.CycloneDXFormat).Extension(),
					Content:   sbom.NewFormattedReader(sbom.SBOM{}, sbom.CycloneDXFormat),
				},
				{
					Extension: sbom.Format(sbom.SPDXFormat).Extension(),
					Content:   sbom.NewFormattedReader(sbom.SBOM{}, sbom.SPDXFormat),
				},
			}))

			Expect(buffer).To(ContainSubstring("Running 'composer install'"))

			Expect(installOptions.DetermineCall.CallCount).To(Equal(1))

			Expect(composerInstallExecution.Args).To(Equal([]string{"install", "options", "from", "fake"}))
			Expect(composerInstallExecution.Dir).To(Equal(filepath.Join(layersDir, composer.ComposerPackagesLayerName)))
			Expect(composerInstallExecution.Stdout).ToNot(BeNil())
			Expect(composerInstallExecution.Stderr).ToNot(BeNil())
			Expect(len(composerInstallExecution.Env)).To(Equal(len(os.Environ()) + 6))

			Expect(sbomGenerator.GenerateCall.Receives.Dir).To(Equal(workingDir))
			Expect(composerInstallExecution.Env).To(ContainElements(
				"COMPOSER_NO_INTERACTION=1",
				fmt.Sprintf("COMPOSER=%s", filepath.Join(workingDir, "composer.json")),
				fmt.Sprintf("COMPOSER_HOME=%s", filepath.Join(layersDir, composer.ComposerPackagesLayerName, ".composer")),
				fmt.Sprintf("COMPOSER_VENDOR_DIR=vendor"),
				fmt.Sprintf("PHPRC=%s", filepath.Join(layersDir, "composer-php-ini", "composer-php.ini")),
				fmt.Sprintf("PATH=fake-path-from-tests")))

			composerPhpIni := filepath.Join(layersDir, "composer-php-ini", "composer-php.ini")
			Expect(composerPhpIni).To(BeARegularFile())
			contentsBytes, err := os.ReadFile(composerPhpIni)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contentsBytes)).To(Equal(`[PHP]
extension_dir = "php-extension-dir"
extension = openssl.so`))

			vendorLink, err := os.Readlink(filepath.Join(workingDir, "vendor"))
			Expect(err).NotTo(HaveOccurred())
			Expect(vendorLink).To(Equal(filepath.Join(layersDir, composer.ComposerPackagesLayerName, "vendor")))
		})

		context("with previously existing vendor dir", func() {
			it.Before(func() {
				Expect(os.Mkdir(filepath.Join(workingDir, "vendor"), os.ModeDir|os.ModePerm)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(workingDir, "vendor", "existing-file.text"), []byte(""), os.ModePerm)).To(Succeed())
			})

			it("copies the vendor dir into the layer for composer install", func() {
				_, err := build(packit.BuildContext{
					BuildpackInfo: buildpackInfo,
					WorkingDir:    workingDir,
					Layers:        packit.Layers{Path: layersDir},
					Plan:          buildpackPlan,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(filepath.Join(layersDir, composer.ComposerPackagesLayerName, "vendor", "existing-file.text")).To(BeARegularFile())
			})
		})
	})

	context("with COMPOSER set", func() {
		it.Before(func() {
			Expect(os.Setenv("COMPOSER", "./foo/bar.file")).To(Succeed())
		})

		it("provides COMPOSER to composer install composerInstallExecution", func() {
			_, err := build(packit.BuildContext{
				BuildpackInfo: buildpackInfo,
				WorkingDir:    workingDir,
				Layers:        packit.Layers{Path: layersDir},
				Plan:          buildpackPlan,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(composerInstallExecution.Env).To(ContainElements(
				fmt.Sprintf("COMPOSER=%s", filepath.Join(workingDir, "foo", "bar.file"))))
			Expect(calculator.SumCall.Receives.Paths).To(Equal([]string{filepath.Join(workingDir, "foo", "composer.lock")}))
		})
	})

	context("with COMPOSER_VENDOR_DIR set", func() {
		it.Before(func() {
			Expect(os.Setenv("COMPOSER_VENDOR_DIR", "some-custom-dir")).To(Succeed())
		})

		it.After(func() {
			Expect(os.Unsetenv("COMPOSER_VENDOR_DIR")).To(Succeed())
		})

		it("symlinks COMPOSER_VENDOR_DIR", func() {
			_, err := build(packit.BuildContext{
				BuildpackInfo: buildpackInfo,
				WorkingDir:    workingDir,
				Layers:        packit.Layers{Path: layersDir},
				Plan:          buildpackPlan,
			})
			Expect(err).NotTo(HaveOccurred())

			vendorLink, err := os.Readlink(filepath.Join(workingDir, "some-custom-dir"))
			Expect(err).NotTo(HaveOccurred())
			Expect(vendorLink).To(Equal(filepath.Join(layersDir, composer.ComposerPackagesLayerName, "vendor")))
		})

		context("with previously existing vendor dir", func() {
			it.Before(func() {
				Expect(os.Mkdir(filepath.Join(workingDir, "some-custom-dir"), os.ModeDir|os.ModePerm)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(workingDir, "some-custom-dir", "existing-file.text"), []byte(""), os.ModePerm)).To(Succeed())
			})

			it("copies the vendor dir into the layer for composer install", func() {
				_, err := build(packit.BuildContext{
					BuildpackInfo: buildpackInfo,
					WorkingDir:    workingDir,
					Layers:        packit.Layers{Path: layersDir},
					Plan:          buildpackPlan,
				})
				Expect(err).NotTo(HaveOccurred())
				existingFile, err := filepath.EvalSymlinks(filepath.Join(workingDir, "some-custom-dir", "existing-file.text"))
				Expect(err).NotTo(HaveOccurred())
				Expect(existingFile).To(BeARegularFile())
				Expect(existingFile).To(HaveSuffix(filepath.Join(composer.ComposerPackagesLayerName, "vendor", "existing-file.text")))
			})
		})
	})

	context("with BP_COMPOSER_INSTALL_GLOBAL", func() {
		it.Before(func() {
			Expect(os.Setenv("BP_COMPOSER_INSTALL_GLOBAL", "friendsofphp/php-cs-fixer squizlabs/php_codesniffer=*")).To(Succeed())
		})

		it.After(func() {
			Expect(os.Unsetenv("BP_COMPOSER_INSTALL_GLOBAL")).To(Succeed())
		})

		it("runs 'composer global require'", func() {
			_, err := build(packit.BuildContext{
				BuildpackInfo: buildpackInfo,
				WorkingDir:    workingDir,
				Layers:        packit.Layers{Path: layersDir},
				Plan:          buildpackPlan,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(composerGlobalExecution.Args).To(Equal([]string{"global", "require", "--no-progress", "friendsofphp/php-cs-fixer", "squizlabs/php_codesniffer=*"}))
			Expect(composerGlobalExecution.Dir).To(Equal(filepath.Join(layersDir, "composer-global")))
			Expect(composerGlobalExecution.Stdout).ToNot(BeNil())
			Expect(composerGlobalExecution.Stderr).ToNot(BeNil())
			Expect(len(composerGlobalExecution.Env)).To(Equal(len(os.Environ()) + 5))

			Expect(composerGlobalExecution.Env).To(ContainElements(
				"COMPOSER_NO_INTERACTION=1",
				fmt.Sprintf("COMPOSER_HOME=%s", filepath.Join(layersDir, "composer-global")),
				fmt.Sprintf("COMPOSER_VENDOR_DIR=vendor"),
				fmt.Sprintf("PHPRC=%s", filepath.Join(layersDir, "composer-php-ini", "composer-php.ini")),
				fmt.Sprintf("PATH=fake-path-from-tests")))

			Expect(composerInstallExecution.Env).To(ContainElements(
				fmt.Sprintf("PATH=%s:fake-path-from-tests", filepath.Join(layersDir, "composer-global", "vendor", "bin"))))
		})
	})

	context("when the checksum for composer.lock matches a previous layer's checksum", func() {
		it.Before(func() {
			buildpackPlan.Entries[0].Metadata["launch"] = true
			buildpackPlan.Entries[0].Metadata["build"] = true
			calculator.SumCall.Returns.String = "sha-from-composer-lock"

			err := os.WriteFile(filepath.Join(layersDir, fmt.Sprintf("%s.toml", composer.ComposerPackagesLayerName)),
				[]byte(`[metadata]
composer-lock-sha = "sha-from-composer-lock"
`), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())

		})

		it("reuses the cached version of the composer packages", func() {
			result, err := build(packit.BuildContext{
				BuildpackInfo: buildpackInfo,
				WorkingDir:    workingDir,
				Layers:        packit.Layers{Path: layersDir},
				Plan:          buildpackPlan,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(buffer).NotTo(ContainSubstring("Running 'composer install'"))

			Expect(calculator.SumCall.Receives.Paths).To(Equal([]string{filepath.Join(workingDir, "composer.lock")}))
			layers := result.Layers
			Expect(layers).To(HaveLen(1))

			packagesLayer := layers[0]
			Expect(packagesLayer.Name).To(Equal(composer.ComposerPackagesLayerName))
			Expect(packagesLayer.Path).To(Equal(filepath.Join(layersDir, composer.ComposerPackagesLayerName)))

			Expect(packagesLayer.Build).To(BeTrue())
			Expect(packagesLayer.Launch).To(BeTrue())
			Expect(packagesLayer.Cache).To(BeTrue())

			Expect(packagesLayer.Metadata["composer-lock-sha"]).To(Equal("sha-from-composer-lock"))

			Expect(packagesLayer.SBOM.Formats()).To(Equal([]packit.SBOMFormat{
				{
					Extension: sbom.Format(sbom.CycloneDXFormat).Extension(),
					Content:   sbom.NewFormattedReader(sbom.SBOM{}, sbom.CycloneDXFormat),
				},
				{
					Extension: sbom.Format(sbom.SPDXFormat).Extension(),
					Content:   sbom.NewFormattedReader(sbom.SBOM{}, sbom.SPDXFormat),
				},
			}))

		})
	})

	context("invokes 'composer check-platform-reqs'", func() {
		it("generates '.php.ini.d/composer-extensions.ini'", func() {
			_, err := build(packit.BuildContext{
				BuildpackInfo: buildpackInfo,
				WorkingDir:    workingDir,
				Layers:        packit.Layers{Path: layersDir},
				Plan:          buildpackPlan,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(composerCheckPlatformReqsExecExecution.Args[0]).To(Equal("check-platform-reqs"))
			Expect(composerCheckPlatformReqsExecExecution.Dir).To(Equal(workingDir))
			Expect(len(composerCheckPlatformReqsExecExecution.Env)).To(Equal(len(os.Environ()) + 3))

			Expect(composerCheckPlatformReqsExecExecution.Env).To(ContainElements(
				"COMPOSER_NO_INTERACTION=1",
				fmt.Sprintf("PHPRC=%s", filepath.Join(layersDir, "composer-php-ini", "composer-php.ini")),
				fmt.Sprintf("PATH=fake-path-from-tests")))

			Expect(filepath.Join(workingDir, ".php.ini.d", "composer-extensions.ini")).To(BeARegularFile())

			contents, err := os.ReadFile(filepath.Join(workingDir, ".php.ini.d", "composer-extensions.ini"))
			Expect(err).NotTo(HaveOccurred())

			Expect(string(contents)).To(Equal(`extension = hello.so
extension = bar.so
`))
		})
	})

	context("with debug logs", func() {
		it.Before(func() {
			Expect(os.Setenv(composer.BpLogLevel, "DEBUG")).To(Succeed())
			Expect(os.Setenv(composer.BpComposerInstallGlobal, "package")).To(Succeed())
		})

		it.After(func() {
			Expect(os.Unsetenv(composer.BpLogLevel)).To(Succeed())
			Expect(os.Unsetenv(composer.BpComposerInstallGlobal)).To(Succeed())
		})

		it("prints additional information", func() {
			_, err := build(packit.BuildContext{
				BuildpackInfo: packit.BuildpackInfo{
					Name:        "buildpack-name",
					Version:     "buildpack-version",
					SBOMFormats: []string{sbom.CycloneDXFormat, sbom.SPDXFormat},
				},
				WorkingDir: workingDir,
				Layers:     packit.Layers{Path: layersDir},
				Plan:       buildpackPlan,
			})
			Expect(err).NotTo(HaveOccurred())
			output := buffer.String()
			Expect(output).To(Equal(fmt.Sprintf(`buildpack-name buildpack-version
  Writing php.ini for composer
    Writing composer-php.ini to %s
    Writing php.ini contents:
    '[PHP]
    extension_dir = "php-extension-dir"
    extension = openssl.so'
  Running 'composer global require'
    stdout from composer global
    stderr from composer global
  Ran 'composer global require --no-progress package'
    Adding global Composer packages to PATH:
    - global-package-name
  Executing build process
  Calculated checksum of default-checksum for composer.lock
  Building new layer %s
    Setting layer types: launch=[true], build=[false], cache=[false]
  Running 'composer install'
    stdout from composer install
    stderr from composer install
  Ran 'composer install options from fake'
      Completed in 0s

  Generating SBOM for %s
      Completed in 0s

  Writing SBOM in the following format(s):
    application/vnd.cyclonedx+json
    application/spdx+json

  Writing symlink %s => %s
    Listing files in %s:
    - local-package-name
  Running 'composer check-platform-reqs'
  Ran 'composer check-platform-reqs', found extensions 'hello, bar'
`,
				filepath.Join(layersDir, composer.ComposerPhpIniLayerName, "composer-php.ini"),
				filepath.Join(layersDir, composer.ComposerPackagesLayerName),
				filepath.Join(layersDir, composer.ComposerPackagesLayerName),
				filepath.Join(workingDir, "vendor"),
				filepath.Join(layersDir, composer.ComposerPackagesLayerName, "vendor"),
				filepath.Join(layersDir, composer.ComposerPackagesLayerName, "vendor"))))
		})
	})

	context("failure cases", func() {
		context("when composerGlobalExecution fails", func() {
			it.Before(func() {
				Expect(os.Setenv(composer.BpComposerInstallGlobal, "anything")).To(Succeed())
				composerGlobalExecutable.ExecuteCall.Stub = func(temp pexec.Execution) error {
					composerGlobalExecution = temp
					_, _ = fmt.Fprint(composerGlobalExecution.Stderr, "error message from global")
					return errors.New("some error from global")
				}
			})

			it.After(func() {
				Expect(os.Unsetenv(composer.BpComposerInstallGlobal)).To(Succeed())
			})

			it("logs the output", func() {
				result, err := build(packit.BuildContext{
					BuildpackInfo: buildpackInfo,
					WorkingDir:    workingDir,
					Layers:        packit.Layers{Path: layersDir},
					Plan:          buildpackPlan,
				})
				Expect(err).To(Equal(errors.New("some error from global")))
				Expect(result).To(Equal(packit.BuildResult{}))

				Expect(buffer.String()).To(ContainSubstring("error message from global"))
			})
		})

		context("when composerCheckPlatformReqsExecution fails", func() {
			it.Before(func() {
				composerCheckPlatformReqsExecExecutable.ExecuteCall.Stub = func(temp pexec.Execution) error {
					composerCheckPlatformReqsExecExecution = temp
					_, _ = fmt.Fprint(composerCheckPlatformReqsExecExecution.Stderr, "error message from check-platform-reqs")
					return errors.New("some error from check-platform-reqs")
				}
			})

			it("logs the output", func() {
				result, err := build(packit.BuildContext{
					BuildpackInfo: buildpackInfo,
					WorkingDir:    workingDir,
					Layers:        packit.Layers{Path: layersDir},
					Plan:          buildpackPlan,
				})
				Expect(err).To(Equal(errors.New("some error from check-platform-reqs")))
				Expect(result).To(Equal(packit.BuildResult{}))

				Expect(buffer.String()).To(ContainSubstring("error message from check-platform-reqs"))
			})
		})

		context("when composerInstallExecution fails", func() {
			it.Before(func() {
				composerInstallExecutable.ExecuteCall.Stub = func(temp pexec.Execution) error {
					composerInstallExecution = temp
					_, _ = fmt.Fprint(composerInstallExecution.Stderr, "error message from install")
					return errors.New("some error from install")
				}
			})

			it("logs the output", func() {
				result, err := build(packit.BuildContext{
					BuildpackInfo: buildpackInfo,
					WorkingDir:    workingDir,
					Layers:        packit.Layers{Path: layersDir},
					Plan:          buildpackPlan,
				})
				Expect(err).To(Equal(errors.New("some error from install")))
				Expect(result).To(Equal(packit.BuildResult{}))

				Expect(buffer.String()).To(ContainSubstring("error message from install"))
			})
		})

		context("when generating the SBOM returns an error", func() {
			it.Before(func() {
				buildpackInfo.SBOMFormats = []string{"random-format"}
			})

			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					BuildpackInfo: buildpackInfo,
					WorkingDir:    workingDir,
					Layers:        packit.Layers{Path: layersDir},
					Plan:          buildpackPlan,
				})
				Expect(err).To(MatchError(`unsupported SBOM format: 'random-format'`))
			})
		})

		context("when formatting the SBOM returns an error", func() {
			it.Before(func() {
				sbomGenerator.GenerateCall.Returns.Error = errors.New("failed to generate SBOM")
			})

			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					BuildpackInfo: buildpackInfo,
					WorkingDir:    workingDir,
					Layers:        packit.Layers{Path: layersDir},
					Plan:          buildpackPlan,
				})
				Expect(err).To(MatchError(ContainSubstring("failed to generate SBOM")))
			})
		})
	})
}
