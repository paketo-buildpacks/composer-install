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
		pack.Build = pack.Build.WithTrustBuilder()
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
			if container.ID != "" {
				Expect(docker.Container.Remove.Execute(container.ID)).To(Succeed())
			}
			if image.ID != "" {
				Expect(docker.Image.Remove.Execute(image.ID)).To(Succeed())
			}
			if name != "" {
				Expect(docker.Volume.Remove.Execute(occam.CacheVolumeNames(name))).To(Succeed())
			}
			if source != "" {
				Expect(os.RemoveAll(source)).To(Succeed())
			}
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
						"BP_PHP_SERVER":     "nginx",
						"BP_PHP_WEB_DIR":    "public",
						"BP_LOG_LEVEL":      "DEBUG",
						"BP_PHP_EXTENSIONS": "mbstring",
					}).
					Execute(name, source)

				Expect(err).ToNot(HaveOccurred(), logs.String)

				container, err = docker.Container.Run.
					WithEnv(map[string]string{
						"PORT": "8080",
						// Route Laravel writable runtime paths to /tmp since /workspace is read-only in CNB containers.
						"LARAVEL_STORAGE_PATH": "/tmp/laravel-storage",
						"VIEW_COMPILED_PATH":   "/tmp/laravel-storage/framework/views",
					}).
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
					WithEnv(map[string]string{
						"PORT": "8080",
						// Redirect Symfony cache/log to /tmp since /workspace is read-only in CNB containers.
						"APP_CACHE_DIR": "/tmp/symfony-cache",
						"APP_LOG_DIR":   "/tmp/symfony-log",
					}).
					WithPublish("8080").
					Execute(image.ID)
				Expect(err).NotTo(HaveOccurred())

				Eventually(container).Should(Serve(ContainSubstring("<title>Symfony Demo application</title>")).OnPort(8080))
				Eventually(container).Should(Serve(ContainSubstring("Symfony Demo blog")).OnPort(8080).WithEndpoint("/en/blog/"))
			})
		})

	})
}
