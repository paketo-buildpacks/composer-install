package fakes

import "sync"

type DetermineComposerInstallOptions struct {
	DetermineCall struct {
		mutex     sync.Mutex
		CallCount int
		Returns   struct {
			StringSlice []string
		}
		Stub func() []string
	}
}

func (f *DetermineComposerInstallOptions) Determine() []string {
	f.DetermineCall.mutex.Lock()
	defer f.DetermineCall.mutex.Unlock()
	f.DetermineCall.CallCount++
	if f.DetermineCall.Stub != nil {
		return f.DetermineCall.Stub()
	}
	return f.DetermineCall.Returns.StringSlice
}
