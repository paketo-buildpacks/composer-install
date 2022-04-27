package composer

import (
	"os"
	"path/filepath"
)

// FindComposerFiles exists to determine where the composer.json and composer.lock files are
// Note that a composer.lock file is not required to exist, but must be a sibling of composer.json
//
// Because it can be helpful during the Detect phase to log why this buildpack will not participate,
// this function will also indicate whether the COMPOSER env var was set.
func FindComposerFiles(workingDir string) (composerJsonPath string, composerLockPath string, composerVar string, composerVarFound bool) {
	composerJsonPath = filepath.Join(workingDir, DefaultComposerJsonPath)
	composerLockPath = filepath.Join(workingDir, DefaultComposerLockPath)

	composerVar, composerVarFound = os.LookupEnv(Composer)
	if composerVarFound {
		composerJsonPath = filepath.Join(workingDir, composerVar)
		composerLockPath = filepath.Join(filepath.Dir(composerJsonPath), DefaultComposerLockPath)
	}

	return
}
