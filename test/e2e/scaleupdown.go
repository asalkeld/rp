package e2e

import (
	"context"
	"crypto/tls"
	"net/http"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/jim-minter/rp/pkg/client/services/preview/redhatopenshift/mgmt/2019-12-31-preview/redhatopenshift"
)

var _ = Describe("Scale Up/Down E2E tests [ScaleUpDown][EveryPR][LongRunning]", func() {
	openshiftclusters := redhatopenshift.NewOpenShiftClustersClientWithBaseURI("https://localhost:8443", os.Getenv("AZURE_SUBSCRIPTION_ID"))
	openshiftclusters.PollingDuration = time.Minute * 90
	openshiftclusters.Sender = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	scale := func(count int32) {
		ctx := context.Background()
		By("Fetching the manifest")
		external, err := openshiftclusters.Get(ctx, os.Getenv("CLUSTER"), os.Getenv("CLUSTER"))
		Expect(err).NotTo(HaveOccurred())

		err = setCount(&external, count)
		Expect(err).NotTo(HaveOccurred())

		By("Calling Update on the rp with the scale up manifest")
		future, err := openshiftclusters.Update(ctx, os.Getenv("CLUSTER"), os.Getenv("CLUSTER"), external)
		Expect(err).NotTo(HaveOccurred())

		By("waiting for cluster to scale")
		err = future.WaitForCompletionRef(ctx, openshiftclusters.Client)
		Expect(err).NotTo(HaveOccurred())
	}

	It("should be possible to maintain a healthy cluster after scaling it out and in", func() {
		By("Scaling up")
		scale(2)

		By("Scaling down")
		scale(1)
	})
})

func setCount(oc *redhatopenshift.OpenShiftCluster, count int32) error {
	for _, p := range *oc.Properties.WorkerProfiles {
		p.Count = &count
	}
	return nil
}
