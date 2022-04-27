package composer

import (
	"os"

	"github.com/mattn/go-shellwords"
)

type InstallOptions struct{}

func NewComposerInstallOptions() InstallOptions {
	return InstallOptions{}
}

// Determine will generate the list of options for `composer install`
// https://getcomposer.org/doc/03-cli.md#install-i
func (_ InstallOptions) Determine() []string {
	if installOptionsFromEnv, exists := os.LookupEnv(BpComposerInstallOptions); !exists {
		return []string{
			"--no-progress",
			"--no-dev",
		}
	} else if installOptionsFromEnv == "" {
		return []string{
			"--no-progress",
		}
	} else {
		parsedOptionsFromEnv, err := shellwords.Parse(installOptionsFromEnv)
		if err != nil {
			return []string{
				"--no-progress",
				installOptionsFromEnv,
			}
		}

		return append([]string{"--no-progress"}, parsedOptionsFromEnv...)
	}
}
