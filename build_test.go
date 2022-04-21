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
	"github.com/paketo-buildpacks/packit/v2/pexec"
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
		calculator                              *fakes.Calculator

		layersDir  string
		workingDir string

		buildpackPlan packit.BuildpackPlan

		build packit.BuildFunc
	)

	it.Before(func() {
		buffer = bytes.NewBuffer(nil)
		installOptions = &fakes.DetermineComposerInstallOptions{}
		composerInstallExecutable = &fakes.Executable{}
		composerGlobalExecutable = &fakes.Executable{}
		composerCheckPlatformReqsExecExecutable = &fakes.Executable{}

		composerInstallExecutable.ExecuteCall.Stub = func(temp pexec.Execution) error {
			composerInstallExecution = temp
			return nil
		}

		composerGlobalExecutable.ExecuteCall.Stub = func(temp pexec.Execution) error {
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

			Expect(err).To(Succeed())

			return nil
		}

		calculator = &fakes.Calculator{}
		calculator.SumCall.Returns.String = "default-checksum"

		var err error
		layersDir, err = os.MkdirTemp("", "layers")
		Expect(err).To(Succeed())

		workingDir, err = os.MkdirTemp("", "working-dir")
		Expect(err).To(Succeed())

		Expect(os.Setenv("PHP_EXTENSION_DIR", "php-extension-dir"))

		installOptions.DetermineCall.Returns.StringSlice = []string{
			"options",
			"from",
			"fake",
		}

		build = composer.Build(scribe.NewEmitter(buffer), installOptions, composerInstallExecutable, composerGlobalExecutable, composerCheckPlatformReqsExecExecutable, "fake-path-from-tests", calculator)

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
			result, err := build(packit.BuildContext{
				WorkingDir: workingDir,
				Layers:     packit.Layers{Path: layersDir},
				Plan:       buildpackPlan,
			})
			Expect(err).To(Succeed())
			Expect(result).To(Equal(packit.BuildResult{
				Layers: []packit.Layer{
					{
						Name:             composer.ComposerPackagesLayerName,
						Path:             filepath.Join(layersDir, composer.ComposerPackagesLayerName),
						SharedEnv:        packit.Environment{},
						BuildEnv:         packit.Environment{},
						LaunchEnv:        packit.Environment{},
						ProcessLaunchEnv: map[string]packit.Environment{},
						Build:            false,
						Launch:           true,
						Cache:            true,
						Metadata: map[string]interface{}{
							"composer-lock-sha": "default-checksum",
						},
					},
				},
			}))

			Expect(buffer).To(ContainSubstring("Running 'composer install'"))

			Expect(installOptions.DetermineCall.CallCount).To(Equal(1))

			Expect(composerInstallExecution.Args).To(Equal([]string{"install", "options", "from", "fake"}))
			Expect(composerInstallExecution.Dir).To(Equal(filepath.Join(layersDir, composer.ComposerPackagesLayerName)))
			Expect(composerInstallExecution.Stdout).ToNot(BeNil())
			Expect(composerInstallExecution.Stderr).ToNot(BeNil())
			Expect(len(composerInstallExecution.Env)).To(Equal(len(os.Environ()) + 6))

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
			Expect(err).To(Succeed())
			Expect(string(contentsBytes)).To(Equal(`[PHP]
extension_dir = "php-extension-dir"
extension = openssl.so`))

			vendorLink, err := os.Readlink(filepath.Join(workingDir, "vendor"))
			Expect(err).To(Succeed())
			Expect(vendorLink).To(Equal(filepath.Join(layersDir, composer.ComposerPackagesLayerName, "vendor")))
		})

		context("with previously existing vendor dir", func() {
			it.Before(func() {
				Expect(os.Mkdir(filepath.Join(workingDir, "vendor"), os.ModeDir|os.ModePerm)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(workingDir, "vendor", "existing-file.text"), []byte(""), os.ModePerm)).To(Succeed())
			})

			it("copies the vendor dir into the layer for composer install", func() {
				_, err := build(packit.BuildContext{
					WorkingDir: workingDir,
					Layers:     packit.Layers{Path: layersDir},
					Plan:       buildpackPlan,
				})
				Expect(err).To(Succeed())
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
				WorkingDir: workingDir,
				Layers:     packit.Layers{Path: layersDir},
				Plan:       buildpackPlan,
			})
			Expect(err).To(Succeed())
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
				WorkingDir: workingDir,
				Layers:     packit.Layers{Path: layersDir},
				Plan:       buildpackPlan,
			})
			Expect(err).To(Succeed())

			vendorLink, err := os.Readlink(filepath.Join(workingDir, "some-custom-dir"))
			Expect(err).To(Succeed())
			Expect(vendorLink).To(Equal(filepath.Join(layersDir, composer.ComposerPackagesLayerName, "vendor")))
		})

		context("with previously existing vendor dir", func() {
			it.Before(func() {
				Expect(os.Mkdir(filepath.Join(workingDir, "some-custom-dir"), os.ModeDir|os.ModePerm)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(workingDir, "some-custom-dir", "existing-file.text"), []byte(""), os.ModePerm)).To(Succeed())
			})

			it("copies the vendor dir into the layer for composer install", func() {
				_, err := build(packit.BuildContext{
					WorkingDir: workingDir,
					Layers:     packit.Layers{Path: layersDir},
					Plan:       buildpackPlan,
				})
				Expect(err).To(Succeed())
				existingFile, err := filepath.EvalSymlinks(filepath.Join(workingDir, "some-custom-dir", "existing-file.text"))
				Expect(err).To(Succeed())
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
				WorkingDir: workingDir,
				Layers:     packit.Layers{Path: layersDir},
				Plan:       buildpackPlan,
			})
			Expect(err).To(Succeed())

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
			Expect(err).To(Succeed())

		})

		it("reuses the cached version of the SDK dependency", func() {
			result, err := build(packit.BuildContext{
				WorkingDir: workingDir,
				Layers:     packit.Layers{Path: layersDir},
				Plan:       buildpackPlan,
			})
			Expect(err).To(Succeed())

			Expect(buffer).NotTo(ContainSubstring("Running 'composer install'"))

			Expect(calculator.SumCall.Receives.Paths).To(Equal([]string{filepath.Join(workingDir, "composer.lock")}))

			Expect(result).To(Equal(packit.BuildResult{
				Layers: []packit.Layer{
					{
						Name:             composer.ComposerPackagesLayerName,
						Path:             filepath.Join(layersDir, composer.ComposerPackagesLayerName),
						SharedEnv:        packit.Environment{},
						BuildEnv:         packit.Environment{},
						LaunchEnv:        packit.Environment{},
						ProcessLaunchEnv: map[string]packit.Environment{},
						Build:            true,
						Launch:           true,
						Cache:            true,
						Metadata: map[string]interface{}{
							"composer-lock-sha": "sha-from-composer-lock",
						},
					},
				},
			}))
		})
	})

	context("invokes 'composer check-platform-reqs'", func() {
		it("generates 'php.ini.d/composer-extensions.ini'", func() {
			_, err := build(packit.BuildContext{
				WorkingDir: workingDir,
				Layers:     packit.Layers{Path: layersDir},
				Plan:       buildpackPlan,
			})
			Expect(err).To(Succeed())

			Expect(composerCheckPlatformReqsExecExecution.Args[0]).To(Equal("check-platform-reqs"))
			Expect(composerCheckPlatformReqsExecExecution.Dir).To(Equal(workingDir))
			Expect(len(composerCheckPlatformReqsExecExecution.Env)).To(Equal(len(os.Environ()) + 4))

			Expect(composerCheckPlatformReqsExecExecution.Env).To(ContainElements(
				"COMPOSER_NO_INTERACTION=1",
				fmt.Sprintf("COMPOSER_HOME=%s", filepath.Join(layersDir, composer.ComposerPackagesLayerName, ".composer")),
				fmt.Sprintf("PHPRC=%s", filepath.Join(layersDir, "composer-php-ini", "composer-php.ini")),
				fmt.Sprintf("PATH=fake-path-from-tests")))

			Expect(filepath.Join(workingDir, "php.ini.d", "composer-extensions.ini")).To(BeARegularFile())

			contents, err := os.ReadFile(filepath.Join(workingDir, "php.ini.d", "composer-extensions.ini"))
			Expect(err).To(Succeed())

			Expect(string(contents)).To(Equal(`extension = hello.so
extension = bar.so
`))
		})
	})

	context("failure cases", func() {
		context("when composerInstallExecution fails", func() {
			it.Before(func() {
				composerInstallExecutable.ExecuteCall.Stub = func(temp pexec.Execution) error {
					composerInstallExecution = temp
					_, _ = fmt.Fprint(composerInstallExecution.Stderr, "error message")
					return errors.New("some error")
				}
			})

			it("logs the output", func() {
				result, err := build(packit.BuildContext{
					WorkingDir: workingDir,
					Layers:     packit.Layers{Path: layersDir},
					Plan:       buildpackPlan,
				})
				Expect(err).To(Equal(errors.New("some error")))
				Expect(result).To(Equal(packit.BuildResult{}))

				Expect(buffer.String()).To(ContainSubstring("error message"))
			})
		})
	})
}
