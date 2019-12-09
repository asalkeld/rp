package e2e

import (
	"context"
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"math/big"
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
	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/jim-minter/rp/pkg/client/services/preview/redhatopenshift/mgmt/2019-12-31-preview/redhatopenshift"
	"github.com/jim-minter/rp/pkg/util/arm"
	"github.com/jim-minter/rp/pkg/util/azureclient/authorization"
	"github.com/jim-minter/rp/pkg/util/azureclient/resources"
	utilpermissions "github.com/jim-minter/rp/pkg/util/permissions"
	utillog "github.com/jim-minter/rp/test/util/log"
)

type clientSet struct {
	location string
	subID    string
	tenantID string
	rg       string

	log *logrus.Entry

	permissions       authorization.PermissionsClient
	roleassignments   authorization.RoleAssignmentsClient
	applications      graphrbac.ApplicationsClient
	deployments       resources.DeploymentsClient
	groups            resources.GroupsClient
	openshiftclusters redhatopenshift.OpenShiftClustersClient
}

func newClientSet(log *logrus.Entry, location, subID, tenantID, rg string) (*clientSet, error) {
	// TODO ask Jim to create this account and insert it into secrets
	conf := auth.NewClientCredentialsConfig(os.Getenv("AZURE_CI_CLIENT_ID"), os.Getenv("AZURE_CI_CLIENT_SECRET"), tenantID)
	authorizer, err := conf.Authorizer()
	if err != nil {
		return nil, err
	}

	conf.Resource = azure.PublicCloud.GraphEndpoint
	graphAuthorizer, err := conf.Authorizer()
	if err != nil {
		return nil, err
	}
	cs := &clientSet{
		log:               log,
		location:          location,
		subID:             subID,
		tenantID:          tenantID,
		rg:                rg,
		permissions:       authorization.NewPermissionsClient(subID, authorizer),
		roleassignments:   authorization.NewRoleAssignmentsClient(subID, authorizer),
		applications:      graphrbac.NewApplicationsClient(tenantID),
		deployments:       resources.NewDeploymentsClient(subID, authorizer),
		groups:            resources.NewGroupsClient(subID, authorizer),
		openshiftclusters: redhatopenshift.NewOpenShiftClustersClient(subID),
	}
	cs.openshiftclusters.Authorizer = authorizer
	cs.applications.Authorizer = graphAuthorizer
	return cs, nil
}

func (c *clientSet) assignVnetRoleTo(ctx context.Context, assigneeID string) error {
	res, err := c.applications.GetServicePrincipalsIDByAppID(ctx, assigneeID)
	if err != nil {
		return err
	}

	_, err = c.roleassignments.Create(ctx, "/subscriptions/"+c.subID+"/resourceGroups/"+c.rg, uuid.NewV4().String(), mgmtauthorization.RoleAssignmentCreateParameters{
		Properties: &mgmtauthorization.RoleAssignmentProperties{
			RoleDefinitionID: to.StringPtr("/subscriptions/" + c.subID + "/providers/Microsoft.Authorization/roleDefinitions/c95361b8-cf7c-40a1-ad0a-df9f39a30225"),
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
	// try removing the code below after a bit if we don't hit the error
	permissions, err := c.permissions.ListForResourceGroup(ctx, c.rg)
	if err != nil {
		return err
	}

	ok, err := utilpermissions.CanDoAction(permissions, "Microsoft.Storage/storageAccounts/write")
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("Microsoft.Storage/storageAccounts/write permission not found")
	}

	return nil
}

func (c *clientSet) ensureResourceGroup(ctx context.Context) error {
	if ready, _ := c.checkResourceGroupIsReady(ctx); ready {
		c.log.Infof("resource group %s already exists", c.rg)
		return nil
	}

	tags := map[string]*string{
		"now": to.StringPtr(fmt.Sprintf("%d", time.Now().Unix())),
		"ttl": to.StringPtr("72h"),
	}

	c.log.Infof("creating resource group %s", c.rg)
	if _, err := c.groups.CreateOrUpdate(ctx, c.rg, azresources.Group{Location: &c.location, Tags: tags}); err != nil {
		return err
	}
	c.log.Infof("waiting for successful provision of resource group %s", c.rg)
	return wait.PollImmediate(5*time.Second, 5*time.Minute, func() (bool, error) {
		return c.checkResourceGroupIsReady(ctx)
	})
}

func (c *clientSet) checkResourceGroupIsReady(ctx context.Context) (bool, error) {
	_, err := c.groups.CheckExistence(ctx, c.rg)
	if err != nil {
		return false, err
	}
	return true, nil
}

func randomAddressPrefix() (string, error) {
	//	10.$((RANDOM & 127)).$((RANDOM & 255)).0/24
	a, err := rand.Int(rand.Reader, big.NewInt(int64(127)))
	if err != nil {
		return "", err
	}
	b, err := rand.Int(rand.Reader, big.NewInt(int64(255)))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("10.%v.%v.0/24", *a, *b), nil
}

func (c *clientSet) createVnet(ctx context.Context, clusterName, vnetName string) error {
	masterPrefix, err := randomAddressPrefix()
	if err != nil {
		return err
	}
	workerPrefix, err := randomAddressPrefix()
	if err != nil {
		return err
	}
	// az network vnet create -g "$VNET_RESOURCEGROUP" -n vnet --address-prefixes 10.0.0.0/9
	//	az network vnet subnet create -g "$VNET_RESOURCEGROUP" --vnet-name vnet -n "$CLUSTER-master" --address-prefixes 10.$((RANDOM & 127)).$((RANDOM & 255)).0/24
	//	az network vnet subnet create -g "$VNET_RESOURCEGROUP" --vnet-name vnet -n "$CLUSTER-worker" --address-prefixes 10.$((RANDOM & 127)).$((RANDOM & 255)).0/24
	t := &arm.Template{
		Schema:         "https://schema.management.azure.com/schemas/2015-01-01/deploymentTemplate.json#",
		ContentVersion: "1.0.0.0",
		Resources: []*arm.Resource{
			{
				Resource: &aznetwork.VirtualNetwork{
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
								Name: to.StringPtr(clusterName + "-master"),
							},
							{
								SubnetPropertiesFormat: &aznetwork.SubnetPropertiesFormat{
									AddressPrefix: to.StringPtr(workerPrefix),
								},
								Name: to.StringPtr(clusterName + "-worker"),
							},
						},
					},
					Name:     to.StringPtr(vnetName),
					Type:     to.StringPtr("Microsoft.Network/virtualNetworks"),
					Location: to.StringPtr(c.location),
				},
			},
		},
	}
	c.log.Infof("creating vnet")
	err = c.deployments.CreateOrUpdateAndWait(ctx, c.rg, "azuredeploy", azresources.Deployment{
		Properties: &azresources.DeploymentProperties{
			Template: t,
			Mode:     azresources.Incremental,
		},
	})
	if err != nil {
		return err
	}
	c.log.Infof("assigning role to AZURE_FP_CLIENT_ID for vnet")
	err = c.assignVnetRoleTo(ctx, os.Getenv("AZURE_FP_CLIENT_ID"))
	if err != nil {
		return err
	}
	c.log.Infof("assigning role to AZURE_CLUSTER_CLIENT_ID for vnet")
	err = c.assignVnetRoleTo(ctx, os.Getenv("AZURE_CLUSTER_CLIENT_ID"))
	if err != nil {
		return err
	}
	return nil
}

func (c *clientSet) createCluster(ctx context.Context, vnetRG, clusterName string) error {
	b, err := ioutil.ReadFile("test/e2e/cluster.json")
	if err != nil {
		return err
	}

	oc := redhatopenshift.OpenShiftCluster{}
	err = oc.UnmarshalJSON(b)
	if err != nil {
		return err
	}
	future, err := c.openshiftclusters.Create(ctx, clusterName, clusterName, oc)
	if err != nil {
		return err
	}

	return future.WaitForCompletionRef(ctx, c.openshiftclusters.Client)
}

var _ = AfterSuite(func() {
	logger := utillog.GetTestLogger()
	cs, err := newClientSet(
		logger,
		os.Getenv("LOCATION"),
		os.Getenv("AZURE_SUBSCRIPTION_ID"),
		os.Getenv("AZURE_TENANT_ID"),
		os.Getenv("VNET_RESOURCEGROUP"))
	if err != nil {
		logger.Fatal(err)
		return
	}

	ctx := context.Background()
	future, err := cs.openshiftclusters.Delete(ctx, os.Getenv("CLUSTER"), os.Getenv("CLUSTER"))
	if err != nil {
		logger.Error(err)
	}
	err = future.WaitForCompletionRef(ctx, cs.openshiftclusters.Client)
	if err != nil {
		logger.Error(err)
	}
	err = cs.groups.DeleteAndWait(ctx, cs.rg)
	if err != nil {
		logger.Error(err)
	}
})

var _ = BeforeSuite(func() {
	logger := utillog.GetTestLogger()
	cs, err := newClientSet(
		logger,
		os.Getenv("LOCATION"),
		os.Getenv("AZURE_SUBSCRIPTION_ID"),
		os.Getenv("AZURE_TENANT_ID"),
		os.Getenv("VNET_RESOURCEGROUP"))
	if err != nil {
		logger.Fatal(err)
		return
	}

	ctx := context.Background()
	err = cs.ensureResourceGroup(ctx)
	if err != nil {
		logger.Fatal(err)
		return
	}
	err = cs.createVnet(ctx, os.Getenv("CLUSTER"), "vnet")
	if err != nil {
		logger.Fatal(err)
		return
	}
	err = cs.createCluster(ctx, os.Getenv("VNET_RESOURCEGROUP"), os.Getenv("CLUSTER"))
	if err != nil {
		logger.Fatal(err)
		return
	}
	// TODO get the kubeconfig and create a client for the tests.
})
