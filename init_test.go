package composer_test

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

func TestUnitComposer(t *testing.T) {
	suite := spec.New("composer", spec.Report(report.Terminal{}))
	suite("Detect", testDetect, spec.Sequential())
	suite("Build", testBuild, spec.Sequential())
	suite("InstallOptions", testComposerInstallOptions)
	suite("PhpVersionResolver", testPhpVersionResolver, spec.Sequential())
	suite.Run(t)
}
