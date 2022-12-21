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

func testFrameworkApps(t *testing.T, context spec.G, it spec.S) {
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

	context("PHP apps that use frameworks", func() {
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

		context("building a laravel app", func() {
			var (
				err  error
				logs fmt.Stringer
			)

			it.Before(func() {
				source, err = occam.Source(filepath.Join("testdata", "laravel_app"))
				Expect(err).NotTo(HaveOccurred())
			})

			it("builds and runs", func() {
				image, logs, err = pack.Build.
					WithPullPolicy("never").
					WithBuildpacks(buildpacksArray...).
					WithEnv(map[string]string{
						"BP_PHP_SERVER":  "nginx",
						"BP_PHP_WEB_DIR": "public",
						"BP_LOG_LEVEL":   "DEBUG",
					}).
					Execute(name, source)

				Expect(err).ToNot(HaveOccurred(), logs.String)

				container, err = docker.Container.Run.
					WithEnv(map[string]string{"PORT": "8080"}).
					WithPublish("8080").
					Execute(image.ID)
				Expect(err).NotTo(HaveOccurred())

				Eventually(container).Should(Serve(ContainSubstring("<title>Laravel</title>")).OnPort(8080))
			})
		})

		context("building a symfony app", func() {
			var (
				err  error
				logs fmt.Stringer
			)

			it.Before(func() {
				source, err = occam.Source(filepath.Join("testdata", "symfony_app"))
				Expect(err).NotTo(HaveOccurred())
			})

			it("builds and runs", func() {
				image, logs, err = pack.Build.
					WithPullPolicy("never").
					WithBuildpacks(buildpacksArray...).
					WithEnv(map[string]string{
						"BP_PHP_SERVER":               "nginx",
						"BP_PHP_WEB_DIR":              "public",
						"BP_COMPOSER_INSTALL_OPTIONS": "--no-scripts -o",
					}).
					Execute(name, source)

				Expect(err).ToNot(HaveOccurred(), logs.String)

				container, err = docker.Container.Run.
					WithEnv(map[string]string{"PORT": "8080"}).
					WithPublish("8080").
					Execute(image.ID)
				Expect(err).NotTo(HaveOccurred())

				Eventually(container).Should(Serve(ContainSubstring("<title>Symfony Demo application</title>")).OnPort(8080))
				Eventually(container).Should(Serve(ContainSubstring("Symfony Demo blog")).OnPort(8080).WithEndpoint("/en/blog/"))
			})
		})

	})
}
