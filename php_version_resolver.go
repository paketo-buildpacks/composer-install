package composer

import (
	"encoding/json"
	"os"

	"github.com/paketo-buildpacks/packit/v2/fs"
)

type PhpVersionResolver struct{}

func NewPhpVersionResolver() PhpVersionResolver {
	return PhpVersionResolver{}
}

func (_ PhpVersionResolver) Resolve(composerJsonPath, composerLockPath string) (version, versionSource string, err error) {
	if exists, err := fs.Exists(composerLockPath); err != nil {
		return "", "", err
	} else if exists {
		file, err := os.Open(composerLockPath)
		defer file.Close()

		var unknownJson map[string]interface{}

		err = json.NewDecoder(file).Decode(&unknownJson)
		if err != nil {
			return "", "", err
		}

		if platform, ok := unknownJson["platform"]; ok {
			switch platform.(type) {
			case []interface{}:
				return "", "", nil
			case map[string]interface{}:
				if platformAsMap, ok := platform.(map[string]interface{}); ok {
					if php64Bit, ok := platformAsMap["php-64bit"].(string); ok {
						return php64Bit, DefaultComposerLockPath, nil
					}
					if php, ok := platformAsMap["php"].(string); ok {
						return php, DefaultComposerLockPath, nil
					}
				}

				return "", "", nil
			}
		}

		return "", "", nil
	} else {
		file, err := os.Open(composerJsonPath)
		if err != nil {
			return "", "", err
		}
		defer file.Close()

		var composerJson struct {
			Require struct {
				Php64bit string `json:"php-64bit"`
				Php      string
			}
		}

		err = json.NewDecoder(file).Decode(&composerJson)
		if err != nil {
			return "", "", err
		}

		err = file.Close()
		if err != nil {
			return "", "", err
		}

		if composerJson.Require.Php64bit != "" {
			return composerJson.Require.Php64bit, DefaultComposerJsonPath, nil
		} else if composerJson.Require.Php != "" {
			return composerJson.Require.Php, DefaultComposerJsonPath, nil
		}
	}

	return
}
