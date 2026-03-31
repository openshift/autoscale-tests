package autonode

import (
	"testing"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAutonodeInit(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Autonode Init Test")
}