package gclient_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestGclient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gclient Suite")
}
