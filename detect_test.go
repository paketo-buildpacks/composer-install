package composer_test

import (
	"bytes"
	"github.com/paketo-buildpacks/composer"
	"github.com/paketo-buildpacks/packit/v2/scribe"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/paketo-buildpacks/packit/v2"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testDetect(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		workingDir string
		buffer     *bytes.Buffer

		detect packit.DetectFunc
	)

	it.Before(func() {
		var err error
		workingDir, err = os.MkdirTemp("", "working-dir")
		Expect(err).NotTo(HaveOccurred())

		buffer = bytes.NewBuffer(nil)
		logEmitter := scribe.NewEmitter(buffer)

		detect = composer.Detect(logEmitter)
	})

	it.After(func() {
		Expect(os.RemoveAll(workingDir)).To(Succeed())
		Expect(os.Unsetenv("COMPOSER")).To(Succeed())
		Expect(os.Unsetenv("BP_COMPOSER_VERSION")).To(Succeed())
		Expect(os.Unsetenv("BP_PHP_VERSION")).To(Succeed())
	})

	context("when BP_PHP_VERSION is set", func() {
		it.Before(func() {
			Expect(os.Setenv("BP_PHP_VERSION", "php.version.from-env")).ToNot(HaveOccurred())
		})

		context("when composer.json is present", func() {
			it.Before(func() {
				Expect(ioutil.WriteFile(filepath.Join(workingDir, "composer.json"), []byte("{}"), 0644)).NotTo(HaveOccurred())
			})

			it(`requires "composer" with version metadata`, func() {
				detectResult, err := detect(packit.DetectContext{WorkingDir: workingDir})
				Expect(err).NotTo(HaveOccurred())

				Expect(detectResult.Plan).To(Equal(packit.BuildPlan{
					Provides: []packit.BuildPlanProvision{},
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
								VersionSource: "BP_PHP_VERSION",
								Version:       "php.version.from-env",
								Build:         true,
							},
						},
					},
				}))
			})
		})
	})

	context("when BP_COMPOSER_VERSION is not set", func() {
		context("when composer.json is present", func() {
			it.Before(func() {
				Expect(ioutil.WriteFile(filepath.Join(workingDir, "composer.json"), []byte("{}"), 0644)).NotTo(HaveOccurred())
			})

			it(`requires "composer" and "php" and provides nothing`, func() {
				detectResult, err := detect(packit.DetectContext{WorkingDir: workingDir})
				Expect(err).NotTo(HaveOccurred())

				Expect(detectResult.Plan).To(Equal(packit.BuildPlan{
					Provides: []packit.BuildPlanProvision{},
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
			})
		})

		context("when $COMPOSER is set", func() {
			it.Before(func() {
				Expect(os.Setenv("COMPOSER", "foobar")).ToNot(HaveOccurred())
			})

			context("when $COMPOSER points to an existing file", func() {
				it.Before(func() {
					Expect(ioutil.WriteFile(filepath.Join(workingDir, "foobar"), []byte("{}"), 0644)).NotTo(HaveOccurred())
				})

				it(`requires "composer" and "php" and provides nothing`, func() {
					detectResult, err := detect(packit.DetectContext{WorkingDir: workingDir})
					Expect(err).NotTo(HaveOccurred())

					Expect(detectResult.Plan).To(Equal(packit.BuildPlan{
						Provides: []packit.BuildPlanProvision{},
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
				})
			})

			context("when $COMPOSER points to an non-existing file", func() {
				it.Before(func() {
					Expect(os.Setenv("COMPOSER", "not-a-real-file")).ToNot(HaveOccurred())
				})

				it(`does not require or provide anything`, func() {
					detectResult, err := detect(packit.DetectContext{WorkingDir: workingDir})
					Expect(err).To(MatchError("no composer.json found at location 'not-a-real-file'"))
					Expect(detectResult.Plan).To(Equal(packit.BuildPlan{}))
				})
			})
		})

		context("when composer.json is not present", func() {
			it(`does not require or provide anything`, func() {
				detectResult, err := detect(packit.DetectContext{WorkingDir: workingDir})
				Expect(err).To(MatchError("no composer.json found"))
				Expect(detectResult.Plan).To(Equal(packit.BuildPlan{}))
			})
		})
	})

	context("when BP_COMPOSER_VERSION is set", func() {
		it.Before(func() {
			Expect(os.Setenv("BP_COMPOSER_VERSION", "composer.version.from-env")).To(Succeed())
		})

		context("when composer.json is present", func() {
			it.Before(func() {
				Expect(ioutil.WriteFile(filepath.Join(workingDir, "composer.json"), []byte("{}"), 0644)).NotTo(HaveOccurred())
			})

			it(`requires "composer" with version metadata`, func() {
				detectResult, err := detect(packit.DetectContext{WorkingDir: workingDir})
				Expect(err).NotTo(HaveOccurred())

				Expect(detectResult.Plan).To(Equal(packit.BuildPlan{
					Provides: []packit.BuildPlanProvision{},
					Requires: []packit.BuildPlanRequirement{
						{
							Name: "composer",
							Metadata: composer.BuildPlanMetadata{
								Version:       "composer.version.from-env",
								VersionSource: "BP_COMPOSER_VERSION",
								Build:         true,
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
			})
		})

		context("when $COMPOSER is set", func() {
			it.Before(func() {
				Expect(os.Setenv("COMPOSER", "foobar")).ToNot(HaveOccurred())
			})

			context("when $COMPOSER points to an existing file", func() {
				it.Before(func() {
					Expect(ioutil.WriteFile(filepath.Join(workingDir, "foobar"), []byte("{}"), 0644)).NotTo(HaveOccurred())
				})

				it(`requires "composer" with version metadata`, func() {
					detectResult, err := detect(packit.DetectContext{WorkingDir: workingDir})
					Expect(err).NotTo(HaveOccurred())

					Expect(detectResult.Plan).To(Equal(packit.BuildPlan{
						Provides: []packit.BuildPlanProvision{},
						Requires: []packit.BuildPlanRequirement{
							{
								Name: "composer",
								Metadata: composer.BuildPlanMetadata{
									Version:       "composer.version.from-env",
									VersionSource: "BP_COMPOSER_VERSION",
									Build:         true,
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
				})
			})

			context("when $COMPOSER points to an non-existing file", func() {
				it.Before(func() {
					Expect(os.Setenv("COMPOSER", "not-a-real-file")).ToNot(HaveOccurred())
				})

				it(`does not require or provide anything`, func() {
					detectResult, err := detect(packit.DetectContext{WorkingDir: workingDir})
					Expect(err).To(MatchError("no composer.json found at location 'not-a-real-file'"))
					Expect(detectResult.Plan).To(Equal(packit.BuildPlan{}))
				})
			})
		})

		context("when composer.json is not present", func() {
			it(`does not require or provide anything`, func() {
				detectResult, err := detect(packit.DetectContext{WorkingDir: workingDir})
				Expect(err).To(MatchError("no composer.json found"))
				Expect(detectResult.Plan).To(Equal(packit.BuildPlan{}))
			})
		})
	})
}
