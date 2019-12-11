package e2e

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
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

var (
	kubeadminPassword string
	apiVersions       = map[string]string{
		"network": "2019-07-01", // copied from 0-installstorage
	}
)

func newClientSet(log *logrus.Entry, location, subID, tenantID, rg string) (*clientSet, error) {
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
		openshiftclusters: redhatopenshift.NewOpenShiftClustersClientWithBaseURI("https://localhost:8443", subID),
	}
	cs.openshiftclusters.PollingDuration = time.Minute * 90
	cs.openshiftclusters.Sender = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	cs.applications.Authorizer = graphAuthorizer
	return cs, nil
}

func (c *clientSet) assignVnetRoleTo(ctx context.Context, assigneeID string) error {
	res, err := c.applications.GetServicePrincipalsIDByAppID(ctx, assigneeID)
	if err != nil {
		return err
	}

	scope := "/subscriptions/" + c.subID + "/resourceGroups/" + c.rg + "/providers/Microsoft.Network/virtualNetworks/vnet"
	_, err = c.roleassignments.Create(ctx, scope, uuid.NewV4().String(), mgmtauthorization.RoleAssignmentCreateParameters{
		Properties: &mgmtauthorization.RoleAssignmentProperties{
			RoleDefinitionID: to.StringPtr("/subscriptions/" + c.subID + "/providers/Microsoft.Authorization/roleDefinitions/f3fe7bc1-0ef9-4681-a68c-c1fa285d6128"),
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
	resp, err := c.groups.CheckExistence(ctx, c.rg)
	if err != nil {
		return false, err
	}
	if resp.Response.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.Response.StatusCode != http.StatusNoContent {
		c.log.Debugf("resp: %v", resp.Response.StatusCode)
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
	return fmt.Sprintf("10.%s.%s.0/24", a.String(), b.String()), nil
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
				APIVersion: apiVersions["network"],
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
	c.log.Debugf("assigning role to AZURE_FP_CLIENT_ID for vnet")
	err = c.assignVnetRoleTo(ctx, os.Getenv("AZURE_FP_CLIENT_ID"))
	if err != nil {
		return err
	}
	c.log.Debugf("assigning role to AZURE_CLUSTER_CLIENT_ID for vnet")
	err = c.assignVnetRoleTo(ctx, os.Getenv("AZURE_CLUSTER_CLIENT_ID"))
	if err != nil {
		return err
	}
	return nil
}

func (c *clientSet) getKubeAdminPassword(ctx context.Context, clusterName string) error {
	c.log.Info("getting kube admin password")
	cred, err := c.openshiftclusters.GetCredentials(ctx, clusterName, clusterName)
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

func (c *clientSet) createCluster(ctx context.Context, vnetRG, clusterName string) error {
	b, err := ioutil.ReadFile("cluster.json")
	if err != nil {
		return err
	}

	oc := redhatopenshift.OpenShiftCluster{}
	err = oc.UnmarshalJSON(b)
	if err != nil {
		return err
	}
	c.log.Infof("creating cluster %s", clusterName)
	future, err := c.openshiftclusters.Create(ctx, clusterName, clusterName, oc)
	if err != nil {
		return err
	}

	c.log.Infof("waiting for cluster %s", clusterName)
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
	if os.Getenv("E2E_NO_DELETE") == "true" {
		logger.Warn("E2E_NO_DELETE set, not deleting the test cluster")
		return
	}

	ctx := context.Background()
	logger.Infof("AfterSuite deleting cluster %s", os.Getenv("CLUSTER"))
	future, err := cs.openshiftclusters.Delete(ctx, os.Getenv("CLUSTER"), os.Getenv("CLUSTER"))
	if err != nil {
		logger.Error(err)
	}
	err = future.WaitForCompletionRef(ctx, cs.openshiftclusters.Client)
	if err != nil {
		logger.Error(err)
	}
	logger.Infof("AfterSuite deleting vnet %s", cs.rg)
	err = cs.deployments.DeleteAndWait(ctx, cs.rg, "azuredeploy")
	if err != nil {
		logger.Error(err)
	}
	logger.Infof("AfterSuite deleting vnet resource group %s", cs.rg)
	err = cs.groups.DeleteAndWait(ctx, cs.rg)
	if err != nil {
		logger.Error(err)
	}
})

var _ = BeforeSuite(func() {
	logger := utillog.GetTestLogger()
	for _, key := range []string{
		"LOCATION", "AZURE_SUBSCRIPTION_ID", "AZURE_TENANT_ID",
		"CLUSTER", "VNET_RESOURCEGROUP",
		"AZURE_ARM_CLIENT_ID", "AZURE_FP_CLIENT_ID", "AZURE_CLUSTER_CLIENT_ID",
		"AZURE_E2E_CLIENT_ID", "AZURE_E2E_CLIENT_SECRET",
	} {
		if _, found := os.LookupEnv(key); !found {
			logger.Errorf("environment variable %q unset", key)
			return
		}
	}

	cs, err := newClientSet(
		logger,
		os.Getenv("LOCATION"),
		os.Getenv("AZURE_SUBSCRIPTION_ID"),
		os.Getenv("AZURE_TENANT_ID"),
		os.Getenv("VNET_RESOURCEGROUP"))
	if err != nil {
		logger.Error(err)
		return
	}
	ctx := context.Background()

	if os.Getenv("E2E_NO_CREATE") == "true" {
		logger.Warn("E2E_NO_CREATE set, not creating the test cluster")
		err = cs.getKubeAdminPassword(ctx, os.Getenv("CLUSTER"))
		if err != nil {
			logger.Error(err)
		}
		return
	}
	logger.Info("BeforeSuite creating the cluster")

	err = cs.ensureResourceGroup(ctx)
	if err != nil {
		logger.Error(err)
		return
	}
	err = cs.createVnet(ctx, os.Getenv("CLUSTER"), "vnet")
	if err != nil {
		logger.Error(err)
		return
	}
	err = cs.createCluster(ctx, os.Getenv("VNET_RESOURCEGROUP"), os.Getenv("CLUSTER"))
	if err != nil {
		logger.Error(err)
		return
	}
	err = cs.getKubeAdminPassword(ctx, os.Getenv("CLUSTER"))
	if err != nil {
		logger.Error(err)
		return
	}
})
