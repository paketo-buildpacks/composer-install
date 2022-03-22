package main

import (
	"github.com/paketo-buildpacks/composer"
	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/scribe"
	"os"
)

func main() {
	logEmitter := scribe.NewEmitter(os.Stdout).WithLevel(os.Getenv("BP_LOG_LEVEL"))
	//dependencyManager := postal.NewService(cargo.NewTransport())
	//entryResolver := draft.NewPlanner()

	packit.Run(
		composer.Detect(
			logEmitter),
		composer.Build(
			logEmitter),
	)
}
