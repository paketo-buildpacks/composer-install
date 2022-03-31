package composer

import (
	"os"
	"path/filepath"
)

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
