package composer

import (
	"os"

	"github.com/mattn/go-shellwords"
)

type InstallOptions struct{}

func NewComposerInstallOptions() InstallOptions {
	return InstallOptions{}
}

func (_ InstallOptions) Determine() []string {
	if installOptonsFromEnv, exists := os.LookupEnv(BpComposerInstallOptions); !exists {
		return []string{
			"--no-progress",
			"--no-dev",
		}
	} else if installOptonsFromEnv == "" {
		return []string{
			"--no-progress",
		}
	} else {
		parsedOptionsFromEnv, err := shellwords.Parse(installOptonsFromEnv)
		if err != nil {
			return []string{
				"--no-progress",
				installOptonsFromEnv,
			}
		}

		return append([]string{"--no-progress"}, parsedOptionsFromEnv...)
	}
}
