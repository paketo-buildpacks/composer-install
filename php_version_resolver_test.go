package composer_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/paketo-buildpacks/composer"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testPhpVersionResolver(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		workingDir string

		phpVersionResolver composer.PhpVersionResolver
	)

	it.Before(func() {
		var err error
		workingDir, err = os.MkdirTemp("", "working-dir")
		Expect(err).NotTo(HaveOccurred())

		phpVersionResolver = composer.NewPhpVersionResolver()
	})

	it.After(func() {
		Expect(os.RemoveAll(workingDir)).To(Succeed())
	})

	context("when both composer.lock and composer.json contain php versions", func() {
		it.Before(func() {
			Expect(ioutil.WriteFile(filepath.Join(workingDir, "composer.json"), []byte(`{
   "require": {
	   "php-64bit": "php-64bit.version.from-composer-json",
	   "php": "php-32bit.version.from-composer-json"
   }
}`), os.ModePerm)).NotTo(HaveOccurred())
		})

		context("when composer.lock contains the 64bit php version", func() {
			it.Before(func() {
				Expect(ioutil.WriteFile(filepath.Join(workingDir, "composer.lock"), []byte(`{
 "platform": {
   "php-64bit": "php-64bit.version.from-composer-lock"
 }
}`), os.ModePerm)).NotTo(HaveOccurred())
			})

			it(`requires "php" with version metadata`, func() {
				version, versionSource, err := phpVersionResolver.Resolve(
					filepath.Join(workingDir, "composer.json"),
					filepath.Join(workingDir, "composer.lock"))
				Expect(err).NotTo(HaveOccurred())
				Expect(version).To(Equal("php-64bit.version.from-composer-lock"))
				Expect(versionSource).To(Equal("composer.lock"))
			})
		})

		context("when composer.lock contains the 32bit php version", func() {
			it.Before(func() {
				Expect(ioutil.WriteFile(filepath.Join(workingDir, "composer.lock"), []byte(`{
 "platform": {
   "php": "php.version.from-composer-lock"
 }
}`), os.ModePerm)).NotTo(HaveOccurred())
			})

			it(`requires "php" with version metadata`, func() {
				version, versionSource, err := phpVersionResolver.Resolve(
					filepath.Join(workingDir, "composer.json"),
					filepath.Join(workingDir, "composer.lock"))
				Expect(err).NotTo(HaveOccurred())
				Expect(version).To(Equal("php.version.from-composer-lock"))
				Expect(versionSource).To(Equal("composer.lock"))
			})
		})

		context("when composer.lock contains both the 32bit and 64bit php version", func() {
			it.Before(func() {
				Expect(ioutil.WriteFile(filepath.Join(workingDir, "composer.lock"), []byte(`{
 "platform": {
   "php-64bit": "php-64bit.version.from-composer-lock",
   "php": "php.version.from-composer-lock"
 }
}`), os.ModePerm)).NotTo(HaveOccurred())
			})

			it(`requires "php" with 64bit version metadata`, func() {
				version, versionSource, err := phpVersionResolver.Resolve(
					filepath.Join(workingDir, "composer.json"),
					filepath.Join(workingDir, "composer.lock"))
				Expect(err).NotTo(HaveOccurred())
				Expect(version).To(Equal("php-64bit.version.from-composer-lock"))
				Expect(versionSource).To(Equal("composer.lock"))
			})
		})
	})

	context("when composer.lock does not have any Platform dependencies", func() {
		it.Before(func() {
			Expect(ioutil.WriteFile(filepath.Join(workingDir, "composer.lock"), []byte(`{
 "platform": []
}`), os.ModePerm)).NotTo(HaveOccurred())
		})

		it(`returns empty versions`, func() {
			version, versionSource, err := phpVersionResolver.Resolve(
				filepath.Join(workingDir, "composer.json"),
				filepath.Join(workingDir, "composer.lock"))
			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal(""))
			Expect(versionSource).To(Equal(""))
		})
	})

	context("when composer.lock is not present", func() {
		context("when composer.json contains the 64bit PHP version", func() {
			it.Before(func() {
				Expect(ioutil.WriteFile(filepath.Join(workingDir, "composer.json"), []byte(`{
	   "require": {
	       "php-64bit": "php-64bit.version.from-composer-json"
	   }
	}`), os.ModePerm)).NotTo(HaveOccurred())
			})

			it(`requires "php" with version metadata`, func() {
				version, versionSource, err := phpVersionResolver.Resolve(
					filepath.Join(workingDir, "composer.json"),
					filepath.Join(workingDir, "composer.lock"))
				Expect(err).NotTo(HaveOccurred())
				Expect(version).To(Equal("php-64bit.version.from-composer-json"))
				Expect(versionSource).To(Equal("composer.json"))
			})
		})

		context("when composer.json contains the 32bit PHP version", func() {
			it.Before(func() {
				Expect(ioutil.WriteFile(filepath.Join(workingDir, "composer.json"), []byte(`{
	   "require": {
	       "php": "php-32bit.version.from-composer-json"
	   }
	}`), os.ModePerm)).NotTo(HaveOccurred())
			})

			it(`requires "php" with version metadata`, func() {
				version, versionSource, err := phpVersionResolver.Resolve(
					filepath.Join(workingDir, "composer.json"),
					filepath.Join(workingDir, "composer.lock"))
				Expect(err).NotTo(HaveOccurred())
				Expect(version).To(Equal("php-32bit.version.from-composer-json"))
				Expect(versionSource).To(Equal("composer.json"))
			})
		})

		context("when composer.json contains both the 64bit and 32bit PHP version", func() {
			it.Before(func() {
				Expect(ioutil.WriteFile(filepath.Join(workingDir, "composer.json"), []byte(`{
	   "require": {
	       "php-64bit": "php-64bit.version.from-composer-json",
	       "php": "php-32bit.version.from-composer-json"
	   }
	}`), os.ModePerm)).NotTo(HaveOccurred())
			})

			it(`requires "php" with version metadata`, func() {
				version, versionSource, err := phpVersionResolver.Resolve(
					filepath.Join(workingDir, "composer.json"),
					filepath.Join(workingDir, "composer.lock"))
				Expect(err).NotTo(HaveOccurred())
				Expect(version).To(Equal("php-64bit.version.from-composer-json"))
				Expect(versionSource).To(Equal("composer.json"))
			})
		})
	})
}
