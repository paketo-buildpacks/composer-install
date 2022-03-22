package integration_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/paketo-buildpacks/occam"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
	. "github.com/paketo-buildpacks/occam/matchers"
)

func testBpComposerVersion(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		pack   occam.Pack
		docker occam.Docker
	)

	it.Before(func() {
		pack = occam.NewPack().WithVerbose()
		docker = occam.NewDocker()
	})

	context("with_bp_composer_version", func() {
		var (
			name   string
			source string
		)

		it.Before(func() {
			var err error
			name, err = occam.RandomName()
			Expect(err).NotTo(HaveOccurred())
		})

		it.After(func() {
			Expect(docker.Volume.Remove.Execute(occam.CacheVolumeNames(name))).To(Succeed())
			Expect(os.RemoveAll(source)).To(Succeed())
		})

		it("recognizes BP_COMPOSER_VERSION as the version source", func() {
			var err error
			var logs fmt.Stringer

			source, err = occam.Source(filepath.Join("testdata", "with_bp_composer_version"))
			Expect(err).NotTo(HaveOccurred())

			_, logs, err = pack.WithNoColor().Build.
				WithPullPolicy("never").
				WithBuildpacks(
					buildpacks.ComposerDist,
					buildpacks.BuildPlan,
				).
				Execute(name, source)
			Expect(err).To(HaveOccurred(), logs.String)

			Expect(logs).To(ContainLines(
				MatchRegexp(fmt.Sprintf(`%s 1\.2\.3`, buildpackInfo.Buildpack.Name)),
				"  Resolving Composer version",
				"    Candidate version sources (in priority order):",
				`      BP_COMPOSER_VERSION -> "9.9.9"`,
			))
			Expect(logs).To(ContainLines(
				MatchRegexp(`\s*failed to satisfy "composer" dependency version constraint "9\.9\.9": no compatible versions on "io\.buildpacks\.stacks\.bionic" stack\. Supported versions are: \[\d\.\d\.\d, \d\.\d\.\d\]`),
			))
		})
	})
}
