package openshiftcluster

import (
	"context"
	"encoding/base64"
	"os"

	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/openshift/installer/pkg/asset/installconfig"
	icazure "github.com/openshift/installer/pkg/asset/installconfig/azure"
	"github.com/openshift/installer/pkg/ipnet"
	"github.com/openshift/installer/pkg/types"
	azuretypes "github.com/openshift/installer/pkg/types/azure"
	openstackvalidation "github.com/openshift/installer/pkg/types/openstack/validation"
	"github.com/openshift/installer/pkg/types/validation"
	"golang.org/x/crypto/ssh"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/jim-minter/rp/pkg/install"
	"github.com/jim-minter/rp/pkg/util/subnet"
)

func (m *Manager) Create(ctx context.Context) error {
	r, err := azure.ParseResourceID(m.doc.OpenShiftCluster.ID)
	if err != nil {
		return err
	}

	vnetID, masterSubnetName, err := subnet.Split(m.doc.OpenShiftCluster.Properties.MasterProfile.SubnetID)
	if err != nil {
		return err
	}

	vnetID, workerSubnetName, err := subnet.Split(m.doc.OpenShiftCluster.Properties.WorkerProfiles[0].SubnetID)
	if err != nil {
		return err
	}

	vnetr, err := azure.ParseResourceID(vnetID)
	if err != nil {
		return err
	}

	sshkey, err := ssh.NewPublicKey(&m.doc.OpenShiftCluster.Properties.SSHKey.PublicKey)
	if err != nil {
		return err
	}

	platformCreds := &installconfig.PlatformCreds{
		Azure: &icazure.Credentials{
			TenantID:       m.doc.OpenShiftCluster.Properties.ServicePrincipalProfile.TenantID,
			ClientID:       m.doc.OpenShiftCluster.Properties.ServicePrincipalProfile.ClientID,
			ClientSecret:   m.doc.OpenShiftCluster.Properties.ServicePrincipalProfile.ClientSecret,
			SubscriptionID: r.SubscriptionID,
		},
		Passthrough: true, // TODO: not working yet
	}

	installConfig := &installconfig.InstallConfig{
		Config: &types.InstallConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: m.doc.OpenShiftCluster.Properties.DomainName,
			},
			SSHKey:     sshkey.Type() + " " + base64.StdEncoding.EncodeToString(sshkey.Marshal()),
			BaseDomain: m.domain,
			Networking: &types.Networking{
				MachineCIDR: ipnet.MustParseCIDR("127.0.0.0/8"), // dummy
				NetworkType: "OpenShiftSDN",
				ClusterNetwork: []types.ClusterNetworkEntry{
					{
						CIDR:       *ipnet.MustParseCIDR(m.doc.OpenShiftCluster.Properties.NetworkProfile.PodCIDR),
						HostPrefix: 23,
					},
				},
				ServiceNetwork: []ipnet.IPNet{
					*ipnet.MustParseCIDR(m.doc.OpenShiftCluster.Properties.NetworkProfile.ServiceCIDR),
				},
			},
			ControlPlane: &types.MachinePool{
				Name:     "master",
				Replicas: to.Int64Ptr(3),
				Platform: types.MachinePoolPlatform{
					Azure: &azuretypes.MachinePool{
						InstanceType: string(m.doc.OpenShiftCluster.Properties.MasterProfile.VMSize),
					},
				},
				Hyperthreading: "Enabled",
			},
			Compute: []types.MachinePool{
				{
					Name:     m.doc.OpenShiftCluster.Properties.WorkerProfiles[0].Name,
					Replicas: to.Int64Ptr(int64(m.doc.OpenShiftCluster.Properties.WorkerProfiles[0].Count)),
					Platform: types.MachinePoolPlatform{
						Azure: &azuretypes.MachinePool{
							InstanceType: string(m.doc.OpenShiftCluster.Properties.WorkerProfiles[0].VMSize),
							OSDisk: azuretypes.OSDisk{
								DiskSizeGB: int32(m.doc.OpenShiftCluster.Properties.WorkerProfiles[0].DiskSizeGB),
							},
						},
					},
					Hyperthreading: "Enabled",
				},
			},
			Platform: types.Platform{
				Azure: &azuretypes.Platform{
					Region:                      m.doc.OpenShiftCluster.Location,
					ResourceGroupName:           m.doc.OpenShiftCluster.Properties.ResourceGroup,
					BaseDomainResourceGroupName: os.Getenv("RESOURCEGROUP"),
					NetworkResourceGroupName:    vnetr.ResourceGroup,
					VirtualNetwork:              vnetr.ResourceName,
					ControlPlaneSubnet:          masterSubnetName,
					ComputeSubnet:               workerSubnetName,
				},
			},
			PullSecret: string(os.Getenv("PULL_SECRET")),
			Publish:    types.ExternalPublishingStrategy,
		},
	}

	err = validation.ValidateInstallConfig(installConfig.Config, openstackvalidation.NewValidValuesFetcher()).ToAggregate()
	if err != nil {
		return err
	}

	return install.NewInstaller(m.log, m.db, m.domain, m.authorizer, r.SubscriptionID).Install(ctx, m.doc, installConfig, platformCreds)
}