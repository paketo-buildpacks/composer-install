package composer_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/paketo-buildpacks/composer"
	"github.com/paketo-buildpacks/composer/fakes"
	"github.com/paketo-buildpacks/packit/v2/scribe"

	"github.com/paketo-buildpacks/packit/v2"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
	. "github.com/paketo-buildpacks/occam/matchers"
)

func testDetect(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		workingDir string
		buffer     *bytes.Buffer

		phpVersionResolver *fakes.PhpVersionResolverInterface

		detect packit.DetectFunc
	)

	it.Before(func() {
		var err error
		workingDir, err = os.MkdirTemp("", "working-dir")
		Expect(err).NotTo(HaveOccurred())

		buffer = bytes.NewBuffer(nil)
		logEmitter := scribe.NewEmitter(buffer)

		phpVersionResolver = &fakes.PhpVersionResolverInterface{}

		detect = composer.Detect(logEmitter, phpVersionResolver)
	})

	it.After(func() {
		Expect(os.RemoveAll(workingDir)).To(Succeed())
		Expect(os.Unsetenv("COMPOSER")).To(Succeed())
		Expect(os.Unsetenv("COMPOSER_VENDOR_DIR")).To(Succeed())
	})

	context("when composer.json is present", func() {
		it.Before(func() {
			Expect(os.WriteFile(filepath.Join(workingDir, "composer.json"), []byte("{}"), 0644)).NotTo(HaveOccurred())
		})

		it(`requires "composer" and "php" and provides "composer-packages"`, func() {
			detectResult, err := detect(packit.DetectContext{WorkingDir: workingDir})
			Expect(err).NotTo(HaveOccurred())

			Expect(detectResult.Plan).To(Equal(packit.BuildPlan{
				Provides: []packit.BuildPlanProvision{
					{
						Name: composer.ComposerPackagesDependency,
					},
				},
				Requires: []packit.BuildPlanRequirement{
					{
						Name: "composer",
						Metadata: composer.BuildPlanMetadata{
							Build: true,
						},
					},
					{
						Name: "php",
						Metadata: composer.BuildPlanMetadata{
							Build: true,
						},
					},
				},
			}))

			Expect(phpVersionResolver.ResolveCall.Receives.ComposerJsonPath).To(Equal(filepath.Join(workingDir, "composer.json")))
			Expect(phpVersionResolver.ResolveCall.Receives.ComposerLockPath).To(Equal(filepath.Join(workingDir, "composer.lock")))
		})

		context("when PhpVersionResolver returns values", func() {
			it.Before(func() {
				phpVersionResolver.ResolveCall.Returns.Version = "php-version-from-resolver"
				phpVersionResolver.ResolveCall.Returns.VersionSource = "php-version-source-from-resolver"
			})

			it(`requires "php" with version and version-source metadata`, func() {
				detectResult, err := detect(packit.DetectContext{WorkingDir: workingDir})
				Expect(err).NotTo(HaveOccurred())

				Expect(detectResult.Plan).To(Equal(packit.BuildPlan{
					Provides: []packit.BuildPlanProvision{
						{
							Name: composer.ComposerPackagesDependency,
						},
					},
					Requires: []packit.BuildPlanRequirement{
						{
							Name: "composer",
							Metadata: composer.BuildPlanMetadata{
								Build: true,
							},
						},
						{
							Name: "php",
							Metadata: composer.BuildPlanMetadata{
								Build:         true,
								Version:       "php-version-from-resolver",
								VersionSource: "php-version-source-from-resolver",
							},
						},
					},
				}))
			})
		})

		context("when composer.lock is not present", func() {
			it("will log a warning", func() {
				_, err := detect(packit.DetectContext{WorkingDir: workingDir})
				Expect(err).NotTo(HaveOccurred())

				Expect(buffer).To(ContainLines("WARNING: Include a 'composer.lock' file with your application! This will make sure the exact same version of dependencies are used when you build. It will also enable caching of your dependency layer."))
			})
		})

		context("failure cases", func() {
			it("will return an error from PhpVersionResolver", func() {
				phpVersionResolver.ResolveCall.Returns.Err = errors.New("some error")

				_, err := detect(packit.DetectContext{WorkingDir: workingDir})
				Expect(err).To(MatchError(errors.New("some error")))
			})
		})

		context("when $COMPOSER_VENDOR_DIR is not underneath the project root", func() {
			invalidPaths := []string{
				"/usr/vendor",
				"../usr/vendor",
				"./../vendor",
			}

			it("does not require or provide anything for invalidPath", func() {
				for _, invalidPath := range invalidPaths {
					Expect(os.Setenv("COMPOSER_VENDOR_DIR", invalidPath))
					_, err := detect(packit.DetectContext{WorkingDir: workingDir})
					Expect(err).To(MatchError(packit.Fail.WithMessage("COMPOSER_VENDOR_DIR must be a relative path underneath the project root")))
				}
			})
		})
	})

	context("when $COMPOSER is set", func() {
		it.Before(func() {
			Expect(os.Setenv("COMPOSER", "other/location/composer.json")).ToNot(HaveOccurred())
		})

		context("when $COMPOSER points to an existing file", func() {
			it.Before(func() {
				Expect(os.Mkdir(filepath.Join(workingDir, "other"), os.ModeDir|os.ModePerm)).NotTo(HaveOccurred())
				Expect(os.Mkdir(filepath.Join(workingDir, "other", "location"), os.ModeDir|os.ModePerm)).NotTo(HaveOccurred())
				Expect(os.WriteFile(filepath.Join(workingDir, "other", "location", "composer.json"), []byte("{}"), os.ModePerm)).NotTo(HaveOccurred())
			})

			it(`requires "composer" and "php" and provides "composer-packages"`, func() {
				detectResult, err := detect(packit.DetectContext{WorkingDir: workingDir})
				Expect(err).NotTo(HaveOccurred())

				Expect(detectResult.Plan).To(Equal(packit.BuildPlan{
					Provides: []packit.BuildPlanProvision{
						{
							Name: composer.ComposerPackagesDependency,
						},
					},
					Requires: []packit.BuildPlanRequirement{
						{
							Name: "composer",
							Metadata: composer.BuildPlanMetadata{
								Build: true,
							},
						},
						{
							Name: "php",
							Metadata: composer.BuildPlanMetadata{
								Build: true,
							},
						},
					},
				}))

				Expect(phpVersionResolver.ResolveCall.Receives.ComposerJsonPath).To(Equal(filepath.Join(workingDir, "other", "location", "composer.json")))
				Expect(phpVersionResolver.ResolveCall.Receives.ComposerLockPath).To(Equal(filepath.Join(workingDir, "other", "location", "composer.lock")))
			})

			context("when composer.lock is not present as a sibling of composer.json", func() {
				it.Before(func() {
					Expect(os.WriteFile(filepath.Join(workingDir, "composer.lock"), []byte("{}"), 0644)).NotTo(HaveOccurred())
				})

				it("will log a warning", func() {
					_, err := detect(packit.DetectContext{WorkingDir: workingDir})
					Expect(err).NotTo(HaveOccurred())

					Expect(buffer).To(ContainLines("WARNING: Include a 'composer.lock' file with your application! This will make sure the exact same version of dependencies are used when you build. It will also enable caching of your dependency layer."))
				})
			})
		})

		context("when $COMPOSER points to an non-existing file", func() {
			it.Before(func() {
				Expect(os.Setenv("COMPOSER", "not-a-real-file")).ToNot(HaveOccurred())
			})

			it(`does not require or provide anything`, func() {
				_, err := detect(packit.DetectContext{WorkingDir: workingDir})
				Expect(err).To(MatchError(packit.Fail.WithMessage("no composer.json found at location 'not-a-real-file'")))
			})
		})
	})

	context("when composer.json is not present", func() {
		it(`does not require or provide anything`, func() {
			_, err := detect(packit.DetectContext{WorkingDir: workingDir})
			Expect(err).To(MatchError(packit.Fail.WithMessage("no composer.json found")))
		})
	})
}
