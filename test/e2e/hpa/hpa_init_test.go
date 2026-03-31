package hpa

import (
	"testing"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestHpaInit(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "HPA Init Test")
}
