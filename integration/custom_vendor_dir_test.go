package integration_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paketo-buildpacks/occam"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
	. "github.com/paketo-buildpacks/occam/matchers"
)

func testCustomVendorDir(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect     = NewWithT(t).Expect
		Eventually = NewWithT(t).Eventually

		pack   occam.Pack
		docker occam.Docker
	)

	it.Before(func() {
		pack = occam.NewPack().WithVerbose().WithNoColor()
		docker = occam.NewDocker()
	})

	context("with custom vendor dir", func() {
		var (
			image     occam.Image
			container occam.Container

			name   string
			source string
		)

		it.Before(func() {
			var err error
			name, err = occam.RandomName()
			Expect(err).NotTo(HaveOccurred())
		})

		it.After(func() {
			Expect(docker.Container.Remove.Execute(container.ID)).To(Succeed())
			Expect(docker.Image.Remove.Execute(image.ID)).To(Succeed())
			Expect(docker.Volume.Remove.Execute(occam.CacheVolumeNames(name))).To(Succeed())
			Expect(os.RemoveAll(source)).To(Succeed())
		})

		it("builds and runs", func() {
			var err error
			var logs fmt.Stringer

			source, err = occam.Source(filepath.Join("testdata", "custom_vendor_dir_app"))
			Expect(err).NotTo(HaveOccurred())

			image, logs, err = pack.Build.
				WithPullPolicy("never").
				WithBuildpacks(buildpacksArray...).
				WithEnv(map[string]string{
					"COMPOSER_VENDOR_DIR": "custom_vendor_dir",
					"BP_LOG_LEVEL":        "DEBUG",
					"BP_PHP_SERVER":       "nginx",
				}).
				Execute(name, source)
			Expect(err).ToNot(HaveOccurred(), logs.String)

			Expect(logs.String()).To(ContainSubstring(fmt.Sprintf("Copying from /workspace/custom_vendor_dir => to /layers/%s/composer-packages/vendor", strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_"))))

			container, err = docker.Container.Run.
				WithEnv(map[string]string{"PORT": "8765"}).
				WithPublish("8765").
				Execute(image.ID)
			Expect(err).NotTo(HaveOccurred())

			Eventually(container).Should(Serve(ContainSubstring("Powered By Paketo Buildpacks")).OnPort(8765))
		})
	})
}
