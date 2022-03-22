package composer_test

import (
	. "github.com/onsi/gomega"
	"github.com/sclevine/spec"
	"testing"
)

func testBuild(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect
	)

	it("works", func() {
		Expect(true).To(Equal(true))
	})
}
