package integration_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/paketo-buildpacks/occam"
	"github.com/paketo-buildpacks/packit/v2/fs"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testReusingLayerRebuild(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		docker occam.Docker
		pack   occam.Pack

		imageIDs     []string
		containerIDs []string

		name   string
		source string
	)

	it.Before(func() {
		var err error
		name, err = occam.RandomName()
		Expect(err).NotTo(HaveOccurred())

		docker = occam.NewDocker()
		pack = occam.NewPack()
		imageIDs = []string{}
		containerIDs = []string{}
	})

	it.After(func() {
		for _, id := range containerIDs {
			Expect(docker.Container.Remove.Execute(id)).To(Succeed())
		}

		for _, id := range imageIDs {
			Expect(docker.Image.Remove.Execute(id)).To(Succeed())
		}

		Expect(docker.Volume.Remove.Execute(occam.CacheVolumeNames(name))).To(Succeed())
		Expect(os.RemoveAll(source)).To(Succeed())
	})

	context("when an app is rebuilt and composer.lock does not change", func() {
		it("reuses a layer from a previous build", func() {
			var (
				err         error
				logs        fmt.Stringer
				firstImage  occam.Image
				secondImage occam.Image
			)

			source, err = occam.Source(filepath.Join("testdata", "default_app"))
			Expect(err).NotTo(HaveOccurred())

			build := pack.WithNoColor().Build.
				WithPullPolicy("never").
				WithBuildpacks(buildpacksArray...).
				WithEnv(map[string]string{
					"BP_PHP_SERVER": "nginx",
				})

			firstImage, logs, err = build.Execute(name, source)
			Expect(err).NotTo(HaveOccurred())

			imageIDs = append(imageIDs, firstImage.ID)

			Expect(firstImage.Buildpacks).To(HaveLen(7))

			Expect(firstImage.Buildpacks[2].Key).To(Equal(buildpackInfo.Buildpack.ID))
			Expect(firstImage.Buildpacks[2].Layers).To(HaveKey("composer-packages"))

			Expect(logs.String()).To(ContainSubstring("Running 'composer install'"))

			// Second pack build
			secondImage, logs, err = build.Execute(name, source)
			Expect(err).NotTo(HaveOccurred())

			imageIDs = append(imageIDs, secondImage.ID)

			Expect(secondImage.Buildpacks).To(HaveLen(7))

			Expect(secondImage.Buildpacks[2].Key).To(Equal(buildpackInfo.Buildpack.ID))
			Expect(secondImage.Buildpacks[2].Layers).To(HaveKey("composer-packages"))

			Expect(logs.String()).NotTo(ContainSubstring("Running 'composer install'"))
			Expect(logs.String()).To(ContainSubstring("Reusing cached layer /layers/paketo-buildpacks_composer-install/composer-packages"))

			Expect(secondImage.Buildpacks[2].Layers["composer-packages"].SHA).To(Equal(firstImage.Buildpacks[2].Layers["composer-packages"].SHA))
		})
	})

	context("when an app is rebuilt and there is a change in composer.lock", func() {
		it("rebuilds the layer", func() {
			var (
				err         error
				logs        fmt.Stringer
				firstImage  occam.Image
				secondImage occam.Image
			)

			source, err = occam.Source(filepath.Join("testdata", "default_app"))
			Expect(err).NotTo(HaveOccurred())

			build := pack.WithNoColor().Build.
				WithPullPolicy("never").
				WithBuildpacks(buildpacksArray...).
				WithEnv(map[string]string{
					"BP_PHP_SERVER": "nginx",
				})

			firstImage, logs, err = build.Execute(name, source)
			Expect(err).NotTo(HaveOccurred())

			imageIDs = append(imageIDs, firstImage.ID)

			Expect(firstImage.Buildpacks).To(HaveLen(7))

			Expect(firstImage.Buildpacks[2].Key).To(Equal(buildpackInfo.Buildpack.ID))
			Expect(firstImage.Buildpacks[2].Layers).To(HaveKey("composer-packages"))

			Expect(logs.String()).To(ContainSubstring("Running 'composer install'"))

			// Second pack build
			Expect(fs.Copy(filepath.Join("testdata", "app_with_no_deps", "composer.json"), filepath.Join(source, "composer.json"))).To(Succeed())
			Expect(fs.Copy(filepath.Join("testdata", "app_with_no_deps", "composer.lock"), filepath.Join(source, "composer.lock"))).To(Succeed())

			secondImage, logs, err = build.
				Execute(name, source)
			Expect(err).NotTo(HaveOccurred())

			imageIDs = append(imageIDs, secondImage.ID)

			Expect(secondImage.Buildpacks).To(HaveLen(7))

			Expect(secondImage.Buildpacks[2].Key).To(Equal(buildpackInfo.Buildpack.ID))
			Expect(secondImage.Buildpacks[2].Layers).To(HaveKey("composer-packages"))

			Expect(logs.String()).To(ContainSubstring("Running 'composer install'"))
			Expect(logs.String()).NotTo(ContainSubstring("Reusing cached layer /layers/paketo-buildpacks_composer-install/composer-packages"))

			Expect(secondImage.Buildpacks[2].Layers["composer-packages"].SHA).NotTo(Equal(firstImage.Buildpacks[2].Layers["composer-packages"].SHA))
		})
	})
}
