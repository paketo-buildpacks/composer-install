package main

import (
	"github.com/paketo-buildpacks/composer"
	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/cargo"
	"github.com/paketo-buildpacks/packit/v2/draft"
	"github.com/paketo-buildpacks/packit/v2/postal"
	"github.com/paketo-buildpacks/packit/v2/scribe"
	"os"
)

func main() {
	logEmitter := scribe.NewEmitter(os.Stdout)
	dependencyManager := postal.NewService(cargo.NewTransport())
	entryResolver := draft.NewPlanner()

	packit.Run(
		composer.Detect(),
		composer.Build(
			logEmitter,
			dependencyManager,
			entryResolver),
	)
}
