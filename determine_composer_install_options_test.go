package composer_test

import (
	"os"
	"testing"

	"github.com/paketo-buildpacks/composer"

	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testComposerInstallOptions(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect  = NewWithT(t).Expect
		options composer.InstallOptions
	)

	it.Before(func() {
		options = composer.NewComposerInstallOptions()
	})

	it.After(func() {
		Expect(os.Unsetenv("BP_COMPOSER_INSTALL_OPTIONS")).To(Succeed())
	})

	context("when BP_COMPOSER_INSTALL_OPTIONS is not set", func() {
		it("should return default options", func() {
			Expect(options.Determine()).To(Equal([]string{
				"--no-progress",
				"--no-dev",
			}))
		})
	})

	context("when BP_COMPOSER_INSTALL_OPTIONS is set to empty", func() {
		it.Before(func() {
			Expect(os.Setenv("BP_COMPOSER_INSTALL_OPTIONS", "")).To(Succeed())
		})

		it("should return --no-progress only", func() {
			Expect(options.Determine()).To(Equal([]string{
				"--no-progress",
			}))
		})
	})

	context("when BP_COMPOSER_INSTALL_OPTIONS has options", func() {
		it.Before(func() {
			Expect(os.Setenv("BP_COMPOSER_INSTALL_OPTIONS", "--foo=bar -v --something")).To(Succeed())
		})

		it("should return those values as individual args", func() {
			Expect(options.Determine()).To(Equal([]string{
				"--no-progress",
				"--foo=bar",
				"-v",
				"--something",
			}))
		})
	})

	context("when BP_COMPOSER_INSTALL_OPTIONS has invalid options", func() {
		it.Before(func() {
			Expect(os.Setenv("BP_COMPOSER_INSTALL_OPTIONS", "invalid'option for composer")).To(Succeed())
		})

		it("should return those values as one single arg", func() {
			Expect(options.Determine()).To(Equal([]string{
				"--no-progress",
				"invalid'option for composer",
			}))
		})
	})
}
