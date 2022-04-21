package integration_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/paketo-buildpacks/occam"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
)

var buildpackInfo struct {
	Buildpack struct {
		ID   string
		Name string
	}
	Metadata struct {
		Dependencies []struct {
			Version string
		}
	}
}

var buildpacks struct {
	PhpDist         string `json:"php-dist"`
	Composer        string `json:"composer"`
	ComposerInstall string
	PhpStart        string `json:"php-start"`
	PhpFpm          string `json:"php-fpm"`
	Nginx           string `json:"nginx"`
	PhpNginx        string `json:"php-nginx"`
}

var buildpacksArray []string

func TestIntegration(t *testing.T) {
	// Do not truncate Gomega matcher output
	// The buildpack output text can be large and we often want to see all of it.
	format.MaxLength = 0

	Expect := NewWithT(t).Expect

	file, err := os.Open("../integration.json")
	Expect(err).To(Succeed())

	Expect(json.NewDecoder(file).Decode(&buildpacks)).To(Succeed())
	Expect(file.Close()).To(Succeed())

	file, err = os.Open("../buildpack.toml")
	Expect(err).To(Succeed())

	_, err = toml.NewDecoder(file).Decode(&buildpackInfo)
	Expect(err).To(Succeed())

	buildpackStore := occam.NewBuildpackStore()

	buildpacks.PhpDist, err = buildpackStore.Get.
		Execute(buildpacks.PhpDist)
	Expect(err).To(Succeed())

	buildpacks.Composer, err = buildpackStore.Get.
		Execute(buildpacks.Composer)
	Expect(err).To(Succeed())

	buildpacks.PhpStart, err = buildpackStore.Get.
		Execute(buildpacks.PhpStart)
	Expect(err).To(Succeed())

	buildpacks.PhpFpm, err = buildpackStore.Get.
		Execute(buildpacks.PhpFpm)
	Expect(err).To(Succeed())

	buildpacks.Nginx, err = buildpackStore.Get.
		Execute(buildpacks.Nginx)
	Expect(err).To(Succeed())

	buildpacks.PhpNginx, err = buildpackStore.Get.
		Execute(buildpacks.PhpNginx)
	Expect(err).To(Succeed())

	root, err := filepath.Abs("./..")
	Expect(err).ToNot(HaveOccurred())

	buildpacks.ComposerInstall, err = buildpackStore.Get.
		WithVersion("1.2.3").
		Execute(root)
	Expect(err).To(Succeed())

	buildpacksArray = []string{
		buildpacks.PhpDist,
		buildpacks.Composer,
		buildpacks.ComposerInstall,
		buildpacks.PhpFpm,
		buildpacks.Nginx,
		buildpacks.PhpNginx,
		buildpacks.PhpStart,
	}

	SetDefaultEventuallyTimeout(5 * time.Second)

	suite := spec.New("Integration", spec.Report(report.Terminal{}))
	suite("Default", testDefaultApp)
	suite("Global", testGlobal)
	suite("CustomVendorDir", testCustomVendorDir)
	suite("WithExtensions", testWithExtensions)
	suite("WithVendoredPackages", testWithVendoredPackages)
	suite("ReusingLayerRebuild", testReusingLayerRebuild, spec.Sequential())
	suite.Run(t)
}
