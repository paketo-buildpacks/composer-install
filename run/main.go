package main

import (
	"os"

	"github.com/paketo-buildpacks/composer"
	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/fs"
	"github.com/paketo-buildpacks/packit/v2/pexec"
	"github.com/paketo-buildpacks/packit/v2/scribe"
)

func main() {
	logEmitter := scribe.NewEmitter(os.Stdout).WithLevel(os.Getenv(composer.BpLogLevel))
	phpVersionResolver := composer.NewPhpVersionResolver()
	options := composer.NewComposerInstallOptions()

	installExec := pexec.NewExecutable("composer")
	globalExec := pexec.NewExecutable("composer")
	checkPlatformReqsExec := pexec.NewExecutable("composer")

	packit.Run(
		composer.Detect(logEmitter, phpVersionResolver),
		composer.Build(
			logEmitter,
			options,
			installExec,
			globalExec,
			checkPlatformReqsExec,
			os.Getenv("PATH"),
			fs.NewChecksumCalculator()),
	)
}
