package e2e

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"time"

	. "github.com/onsi/ginkgo"

	mgmtauthorization "github.com/Azure/azure-sdk-for-go/services/authorization/mgmt/2015-07-01/authorization"
	"github.com/Azure/azure-sdk-for-go/services/graphrbac/1.6/graphrbac"
	aznetwork "github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-07-01/network"
	azresources "github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-05-01/resources"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/Azure/go-autorest/autorest/to"
	azredhatopenshift "github.com/jim-minter/rp/pkg/client/services/preview/redhatopenshift/mgmt/2019-12-31-preview/redhatopenshift"
	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"

	"github.com/jim-minter/rp/pkg/util/azureclient/authorization"
	"github.com/jim-minter/rp/pkg/util/azureclient/network"
	"github.com/jim-minter/rp/pkg/util/azureclient/redhatopenshift"
	"github.com/jim-minter/rp/pkg/util/azureclient/resources"
	utillog "github.com/jim-minter/rp/test/util/log"
)

type ClientSet struct {
	Location            string
	SubscriptionID      string
	TenantID            string
	VnetRG              string
	ClusterName         string
	ClusterRG           string
	ClusterClientID     string
	ClusterClientSecret string

	log *logrus.Entry

	permissions       authorization.PermissionsClient
	roleassignments   authorization.RoleAssignmentsClient
	applications      graphrbac.ApplicationsClient
	groups            resources.GroupsClient
	virtualNetworks   network.VirtualNetworksClient
	openshiftclusters redhatopenshift.OpenShiftClustersClient
}

var (
	Clients           *ClientSet
	kubeadminPassword string
)

func newClientSet(log *logrus.Entry) (*ClientSet, error) {
	subID := os.Getenv("AZURE_SUBSCRIPTION_ID")
	tenantID := os.Getenv("AZURE_TENANT_ID")
	vnetRG := os.Getenv("VNET_RESOURCEGROUP")

	conf := auth.NewClientCredentialsConfig(os.Getenv("AZURE_E2E_CLIENT_ID"), os.Getenv("AZURE_E2E_CLIENT_SECRET"), tenantID)
	authorizer, err := conf.Authorizer()
	if err != nil {
		return nil, err
	}

	conf.Resource = azure.PublicCloud.GraphEndpoint
	graphAuthorizer, err := conf.Authorizer()
	if err != nil {
		return nil, err
	}

	cs := &ClientSet{
		log:                 log,
		Location:            os.Getenv("LOCATION"),
		SubscriptionID:      subID,
		TenantID:            tenantID,
		VnetRG:              vnetRG,
		ClusterName:         os.Getenv("CLUSTER"),
		ClusterRG:           os.Getenv("RESOURCEGROUP"),
		ClusterClientID:     os.Getenv("AZURE_CLUSTER_CLIENT_ID"),
		ClusterClientSecret: os.Getenv("AZURE_CLUSTER_CLIENT_SECRET"),
		permissions:         authorization.NewPermissionsClient(subID, authorizer),
		roleassignments:     authorization.NewRoleAssignmentsClient(subID, authorizer),
		applications:        graphrbac.NewApplicationsClient(tenantID),
		groups:              resources.NewGroupsClient(subID, authorizer),
		virtualNetworks:     network.NewVirtualNetworksClient(subID, authorizer),
		openshiftclusters:   redhatopenshift.NewOpenShiftClustersClient(subID, authorizer),
	}

	cs.applications.Authorizer = graphAuthorizer
	return cs, nil
}

func (c *ClientSet) assignVnetRoleTo(ctx context.Context, assigneeID string) error {
	res, err := c.applications.GetServicePrincipalsIDByAppID(ctx, assigneeID)
	if err != nil {
		return err
	}

	scope := "/subscriptions/" + c.SubscriptionID + "/resourceGroups/" + c.VnetRG + "/providers/Microsoft.Network/virtualNetworks/vnet"
	_, err = c.roleassignments.Create(ctx, scope, uuid.NewV4().String(), mgmtauthorization.RoleAssignmentCreateParameters{
		Properties: &mgmtauthorization.RoleAssignmentProperties{
			RoleDefinitionID: to.StringPtr("/subscriptions/" + c.SubscriptionID + "/providers/Microsoft.Authorization/roleDefinitions/f3fe7bc1-0ef9-4681-a68c-c1fa285d6128"),
			PrincipalID:      res.Value,
		},
	})
	if err != nil {
		var ignore bool
		if err, ok := err.(autorest.DetailedError); ok {
			if err, ok := err.Original.(*azure.RequestError); ok && err.ServiceError != nil && err.ServiceError.Code == "RoleAssignmentExists" {
				ignore = true
			}
		}
		if !ignore {
			return err
		}
	}
	return nil
}

func (c *ClientSet) ensureResourceGroup(ctx context.Context) error {
	tags := map[string]*string{
		"now": to.StringPtr(fmt.Sprintf("%d", time.Now().Unix())),
		"ttl": to.StringPtr("72h"),
	}

	c.log.Infof("creating resource group %s", c.VnetRG)
	if _, err := c.groups.CreateOrUpdate(ctx, c.VnetRG, azresources.Group{Location: &c.Location, Tags: tags}); err != nil {
		return err
	}
	c.log.Infof("creating resource group %s", c.ClusterRG)
	if _, err := c.groups.CreateOrUpdate(ctx, c.ClusterRG, azresources.Group{Location: &c.Location, Tags: tags}); err != nil {
		return err
	}
	return nil
}

func randomAddressPrefix() string {
	//	10.$((RANDOM & 127)).$((RANDOM & 255)).0/24
	return fmt.Sprintf("10.%v.%v.0/24", rand.Int63n(int64(127)), rand.Int63n(int64(255)))
}

func (c *ClientSet) createVnet(ctx context.Context, vnetName string) error {
	masterPrefix := randomAddressPrefix()
	workerPrefix := randomAddressPrefix()
	params := aznetwork.VirtualNetwork{
		VirtualNetworkPropertiesFormat: &aznetwork.VirtualNetworkPropertiesFormat{
			AddressSpace: &aznetwork.AddressSpace{
				AddressPrefixes: &[]string{
					"10.0.0.0/9",
				},
			},
			Subnets: &[]aznetwork.Subnet{
				{
					SubnetPropertiesFormat: &aznetwork.SubnetPropertiesFormat{
						AddressPrefix: to.StringPtr(masterPrefix),
					},
					Name: to.StringPtr(c.ClusterName + "-master"),
				},
				{
					SubnetPropertiesFormat: &aznetwork.SubnetPropertiesFormat{
						AddressPrefix: to.StringPtr(workerPrefix),
					},
					Name: to.StringPtr(c.ClusterName + "-worker"),
				},
			},
		},
		Name:     to.StringPtr(vnetName),
		Type:     to.StringPtr("Microsoft.Network/virtualNetworks"),
		Location: to.StringPtr(c.Location),
	}

	c.log.Infof("creating vnet")
	err := c.virtualNetworks.CreateOrUpdateAndWait(ctx, c.VnetRG, vnetName, params)
	if err != nil {
		return err
	}

	rpClientID := "f1dd0a37-89c6-4e07-bcd1-ffd3d43d8875"
	if os.Getenv("RP_MODE") == "development" {
		rpClientID = os.Getenv("AZURE_FP_CLIENT_ID")
	}
	c.log.Debugf("assigning role to AZURE_FP_CLIENT_ID for vnet")
	err = c.assignVnetRoleTo(ctx, rpClientID)
	if err != nil {
		return err
	}
	c.log.Debugf("assigning role to AZURE_CLUSTER_CLIENT_ID for vnet")
	err = c.assignVnetRoleTo(ctx, c.ClusterClientID)
	if err != nil {
		return err
	}
	return nil
}

func (c *ClientSet) getKubeAdminPassword(ctx context.Context) error {
	c.log.Info("getting kube admin password")
	cred, err := c.openshiftclusters.GetCredentials(ctx, c.ClusterRG, c.ClusterName)
	if err != nil {
		return err
	}
	if cred.KubeadminPassword != nil {
		kubeadminPassword = *cred.KubeadminPassword
	} else {
		c.log.Warnf("KubeadminPassword is empty")
	}
	return nil
}

func (c *ClientSet) createCluster(ctx context.Context) error {
	oc := azredhatopenshift.OpenShiftCluster{
		ID:       to.StringPtr(fmt.Sprintf("/subscriptions/%s/resourcegroups/%s/providers/Microsoft.RedHatOpenShift/openShiftClusters/%s", c.SubscriptionID, c.ClusterRG, c.ClusterName)),
		Name:     to.StringPtr(c.ClusterName),
		Type:     to.StringPtr("Microsoft.RedHatOpenShift/openShiftClusters"),
		Location: to.StringPtr(c.Location),
		Properties: &azredhatopenshift.Properties{

			ServicePrincipalProfile: &azredhatopenshift.ServicePrincipalProfile{
				ClientID:     to.StringPtr(c.ClusterClientID),
				ClientSecret: to.StringPtr(c.ClusterClientSecret),
			},
			NetworkProfile: &azredhatopenshift.NetworkProfile{
				PodCidr:     to.StringPtr("10.128.0.0/14"),
				ServiceCidr: to.StringPtr("172.30.0.0/16"),
			},
			MasterProfile: &azredhatopenshift.MasterProfile{
				VMSize:   azredhatopenshift.StandardD8sV3,
				SubnetID: to.StringPtr(fmt.Sprintf("/subscriptions/%s/resourcegroups/%s/providers/Microsoft.Network/virtualNetworks/vnet/subnets/%s-master", c.SubscriptionID, c.VnetRG, c.ClusterName)),
			},
			WorkerProfiles: &[]azredhatopenshift.WorkerProfile{
				{
					Name:       to.StringPtr("worker"),
					VMSize:     azredhatopenshift.VMSize1StandardD2sV3,
					DiskSizeGB: to.Int32Ptr(128),
					SubnetID:   to.StringPtr(fmt.Sprintf("/subscriptions/%s/resourcegroups/%s/providers/Microsoft.Network/virtualNetworks/vnet/subnets/%s-worker", c.SubscriptionID, c.VnetRG, c.ClusterName)),
					Count:      to.Int32Ptr(3),
				},
			},
		},
	}
	c.log.Infof("creating cluster %s/%s", c.ClusterRG, c.ClusterName)
	return c.openshiftclusters.CreateOrUpdateAndWait(ctx, c.ClusterRG, c.ClusterName, oc)
}

var _ = AfterSuite(func() {
	var err error
	logger := utillog.GetTestLogger()
	Clients, err = newClientSet(logger)
	if err != nil {
		panic(err)
	}
	if os.Getenv("E2E_NO_DELETE") == "true" {
		logger.Warn("E2E_NO_DELETE set, not deleting the test cluster")
		return
	}

	ctx := context.Background()
	logger.Infof("AfterSuite deleting cluster %s", Clients.ClusterName)
	err = Clients.openshiftclusters.DeleteAndWait(ctx, Clients.ClusterRG, Clients.ClusterName)
	if err != nil {
		logger.Error(err)
	}
	logger.Infof("AfterSuite deleting cluster resource group %s", Clients.ClusterRG)
	err = Clients.groups.DeleteAndWait(ctx, Clients.ClusterRG)
	if err != nil {
		logger.Error(err)
	}
	logger.Infof("AfterSuite deleting vnet %s", Clients.VnetRG)
	err = Clients.virtualNetworks.DeleteAndWait(ctx, Clients.VnetRG, "vnet")
	if err != nil {
		logger.Error(err)
	}
	logger.Infof("AfterSuite deleting vnet resource group %s", Clients.VnetRG)
	err = Clients.groups.DeleteAndWait(ctx, Clients.VnetRG)
	if err != nil {
		logger.Error(err)
	}
})

var _ = BeforeSuite(func() {
	logger := utillog.GetTestLogger()
	for _, key := range []string{
		"LOCATION", "AZURE_SUBSCRIPTION_ID", "AZURE_TENANT_ID",
		"CLUSTER", "RESOURCEGROUP", "VNET_RESOURCEGROUP",
		"AZURE_FP_CLIENT_ID", "AZURE_CLUSTER_CLIENT_ID",
		"AZURE_E2E_CLIENT_ID", "AZURE_E2E_CLIENT_SECRET",
	} {
		if _, found := os.LookupEnv(key); !found {
			panic(fmt.Sprintf("environment variable %q unset", key))
		}
	}
	var err error
	Clients, err = newClientSet(logger)
	if err != nil {
		panic(err)
	}
	ctx := context.Background()

	if os.Getenv("E2E_NO_CREATE") == "true" {
		logger.Warn("E2E_NO_CREATE set, not creating the test cluster")
		err = Clients.getKubeAdminPassword(ctx)
		if err != nil {
			panic(err)
		}
	}
	logger.Info("BeforeSuite creating the cluster")

	err = Clients.ensureResourceGroup(ctx)
	if err != nil {
		panic(err)
	}
	err = Clients.createVnet(ctx, "vnet")
	if err != nil {
		panic(err)
	}
	err = Clients.createCluster(ctx)
	if err != nil {
		panic(err)
	}
	err = Clients.getKubeAdminPassword(ctx)
	if err != nil {
		panic(err)
	}
})
