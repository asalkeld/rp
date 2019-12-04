package mocks

import (
	"fmt"

	uuid "github.com/satori/go.uuid"

	"github.com/jim-minter/rp/pkg/api"
)

func MockOpenShiftCluster() *api.OpenShiftCluster {
	subID := uuid.NewV4().String()
	testLocation := "australiasoutheast"
	clusterName := "test-cluster"
	vnetName := "test-vnet"
	subnetName := "test-subnet"
	rg := "testrg"
	resID := fmt.Sprintf("/subscriptions/%s/resourcegroups/%s/providers/Microsoft.RedHatOpenShift/openShiftClusters/%s", subID, rg, clusterName)
	vnetID := fmt.Sprintf("/subscriptions/%s/resourcegroups/%s/providers/Microsoft.Network/virtualNetworks/%s", subID, rg, vnetName)
	subnetID := vnetID + "/subnets/" + subnetName
	return &api.OpenShiftCluster{
		ID:       resID,
		Name:     clusterName,
		Type:     "Microsoft.RedHatOpenShift/openShiftClusters",
		Location: testLocation,
		Properties: api.Properties{
			StorageSuffix: "keep",
			DomainName:    "dommy",
			APIServerURL:  "url",
			ConsoleURL:    "url",
			SSHKey:        DummyPrivateKey,
			ServicePrincipalProfile: api.ServicePrincipalProfile{
				ClientID:     uuid.NewV4().String(),
				ClientSecret: "hidden",
			},
			NetworkProfile: api.NetworkProfile{
				PodCIDR:     "10.0.0.0/18",
				ServiceCIDR: "10.0.1.0/22",
			},
			MasterProfile: api.MasterProfile{
				VMSize:   api.VMSizeStandardD8sV3,
				SubnetID: subnetID,
			},
			WorkerProfiles: []api.WorkerProfile{
				{
					Name:       "worker",
					VMSize:     api.VMSizeStandardD4sV3,
					DiskSizeGB: 128,
					SubnetID:   subnetID,
					Count:      12,
				},
			},
		},
	}
}
