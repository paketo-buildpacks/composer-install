package fakes

import (
	"sync"
)

type PhpVersionResolverInterface struct {
	ResolveCall struct {
		mutex     sync.Mutex
		CallCount int
		Receives  struct {
			ComposerJsonPath string
			ComposerLockPath string
		}
		Returns struct {
			Version       string
			VersionSource string
			Err           error
		}
		Stub func(string, string) (string, string, error)
	}
}

func (f *PhpVersionResolverInterface) Resolve(param1 string, param2 string) (string, string, error) {
	f.ResolveCall.mutex.Lock()
	defer f.ResolveCall.mutex.Unlock()
	f.ResolveCall.CallCount++
	f.ResolveCall.Receives.ComposerJsonPath = param1
	f.ResolveCall.Receives.ComposerLockPath = param2
	if f.ResolveCall.Stub != nil {
		return f.ResolveCall.Stub(param1, param2)
	}
	return f.ResolveCall.Returns.Version, f.ResolveCall.Returns.VersionSource, f.ResolveCall.Returns.Err
}
