package e2e

import (
	"context"

	"github.com/Azure/go-autorest/autorest/to"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/jim-minter/rp/pkg/client/services/preview/redhatopenshift/mgmt/2019-12-31-preview/redhatopenshift"
)

var _ = Describe("Scale Up/Down E2E tests [ScaleUpDown][EveryPR][LongRunning]", func() {
	scale := func(count int32) {
		ctx := context.Background()
		external := redhatopenshift.OpenShiftCluster{
			Properties: &redhatopenshift.Properties{
				WorkerProfiles: &[]redhatopenshift.WorkerProfile{
					{
						Count: to.Int32Ptr(count),
					},
				},
			},
		}

		By("Calling Update on the rp with the scale up manifest")
		err := Clients.openshiftclusters.UpdateAndWait(ctx, Clients.ClusterRG, Clients.ClusterName, external)
		Expect(err).NotTo(HaveOccurred())
	}

	It("should be possible to maintain a healthy cluster after scaling it out and in", func() {
		By("Scaling up")
		scale(4)

		By("Scaling down")
		scale(3)
	})
})

func setCount(oc *redhatopenshift.OpenShiftCluster, count int32) error {
	for _, p := range *oc.Properties.WorkerProfiles {
		p.Count = &count
	}
	return nil
}
