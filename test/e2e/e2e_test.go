//+build e2e

package e2e

import (
	"flag"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/format"

	utillog "github.com/jim-minter/rp/test/util/log"
)

var (
	gitCommit = "unknown"
)

func TestE2E(t *testing.T) {
	flag.Parse()
	log := utillog.GetTestLogger()
	log.Infof("e2e tests starting, git commit %s\n", gitCommit)
	RegisterFailHandler(Fail)
	format.TruncatedDiff = false
	RunSpecs(t, "e2e tests")
}
