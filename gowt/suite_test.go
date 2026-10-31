package gowt_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestGowt(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gowt Suite")
}
