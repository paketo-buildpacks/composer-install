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

func testStackUpgrade(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect     = NewWithT(t).Expect
		Eventually = NewWithT(t).Eventually

		docker occam.Docker
		pack   occam.Pack

		imageIDs     map[string]struct{}
		containerIDs map[string]struct{}

		name   string
		source string
	)

	it.Before(func() {
		var err error
		name, err = occam.RandomName()
		Expect(err).NotTo(HaveOccurred())

		docker = occam.NewDocker()
		pack = occam.NewPack()
		imageIDs = map[string]struct{}{}
		containerIDs = map[string]struct{}{}

		// pull images associated with the jammy builder incase they haven't been pulled yet
		Expect(docker.Pull.Execute("paketobuildpacks/builder-jammy-buildpackless-full:latest")).To(Succeed())
		Expect(docker.Pull.Execute("paketobuildpacks/run-jammy-full:latest")).To(Succeed())
		jammyBuilder, err := pack.Builder.Inspect.Execute("paketobuildpacks/builder-jammy-buildpackless-full")
		Expect(err).NotTo(HaveOccurred())
		Expect(docker.Pull.Execute(
			fmt.Sprintf("%s:%s", "buildpacksio/lifecycle", jammyBuilder.RemoteInfo.Lifecycle.Version),
		)).To(Succeed())
	})

	it.After(func() {
		for id := range containerIDs {
			Expect(docker.Container.Remove.Execute(id)).To(Succeed())
		}

		for id := range imageIDs {
			Expect(docker.Image.Remove.Execute(id)).To(Succeed())
		}

		Expect(docker.Image.Remove.Execute("paketobuildpacks/builder-jammy-buildpackless-full:latest")).To(Succeed())
		Expect(docker.Image.Remove.Execute("paketobuildpacks/run-jammy-full:latest")).To(Succeed())

		Expect(docker.Volume.Remove.Execute(occam.CacheVolumeNames(name))).To(Succeed())
		Expect(os.RemoveAll(source)).To(Succeed())
	})

	context("when an app is rebuilt and the underlying stack changes", func() {
		it("rebuilds the packages layer", func() {
			var (
				err         error
				logs        fmt.Stringer
				firstImage  occam.Image
				secondImage occam.Image

				firstContainer  occam.Container
				secondContainer occam.Container
			)

			source, err = occam.Source(filepath.Join("testdata", "default_app"))
			Expect(err).NotTo(HaveOccurred())

			build := pack.WithNoColor().Build.
				WithPullPolicy("never").
				WithEnv(map[string]string{
					"BP_PHP_SERVER": "nginx",
				}).
				WithBuildpacks(buildpacksArray...)

			firstImage, logs, err = build.Execute(name, source)
			Expect(err).NotTo(HaveOccurred(), logs.String())
			Expect(logs).To(ContainSubstring("Running 'composer install --no-progress --no-dev'"))

			imageIDs[firstImage.ID] = struct{}{}

			firstContainer, err = docker.Container.Run.
				WithEnv(map[string]string{"PORT": "8765"}).
				WithPublish("8765").
				Execute(firstImage.ID)
			Expect(err).NotTo(HaveOccurred())

			containerIDs[firstContainer.ID] = struct{}{}
			Eventually(firstContainer).Should(Serve(ContainSubstring("Powered By Paketo Buildpacks")).OnPort(8765))

			// Second pack build, upgrade stack image
			secondImage, logs, err = build.WithBuilder("paketobuildpacks/builder-jammy-buildpackless-full").Execute(name, source)
			Expect(err).NotTo(HaveOccurred())

			imageIDs[secondImage.ID] = struct{}{}
			Expect(logs).To(ContainSubstring("Running 'composer install --no-progress --no-dev'"))
			Expect(logs.String()).NotTo(ContainSubstring(fmt.Sprintf("Reusing cached layer /layers/%s/composer-packages", strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_"))))

			imageIDs[secondImage.ID] = struct{}{}

			secondContainer, err = docker.Container.Run.
				WithEnv(map[string]string{"PORT": "8765"}).
				WithPublish("8765").
				Execute(secondImage.ID)
			Expect(err).NotTo(HaveOccurred())

			containerIDs[secondContainer.ID] = struct{}{}
			Eventually(secondContainer).Should(Serve(ContainSubstring("Powered By Paketo Buildpacks")).OnPort(8765))
		})
	})
}
