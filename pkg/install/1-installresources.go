package install

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"reflect"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/authorization/mgmt/2015-07-01/authorization"
	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-03-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/dns/mgmt/2018-05-01/dns"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-07-01/network"
	"github.com/Azure/azure-sdk-for-go/services/privatedns/mgmt/2018-09-01/privatedns"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-05-01/resources"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/apparentlymart/go-cidr/cidr"
	"github.com/openshift/installer/pkg/asset/ignition/machine"
	"github.com/openshift/installer/pkg/asset/installconfig"
	"github.com/openshift/installer/pkg/asset/machines"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/jim-minter/rp/pkg/api"
	"github.com/jim-minter/rp/pkg/util/arm"
	"github.com/jim-minter/rp/pkg/util/restconfig"
	"github.com/jim-minter/rp/pkg/util/subnet"
)

func (i *Installer) installResources(ctx context.Context, doc *api.OpenShiftClusterDocument) error {
	g, err := i.getGraph(ctx, doc.OpenShiftCluster)
	if err != nil {
		return err
	}

	installConfig := g[reflect.TypeOf(&installconfig.InstallConfig{})].(*installconfig.InstallConfig)
	machinesMaster := g[reflect.TypeOf(&machines.Master{})].(*machines.Master)
	machineMaster := g[reflect.TypeOf(&machine.Master{})].(*machine.Master)

	vnetID, _, err := subnet.Split(doc.OpenShiftCluster.Properties.MasterProfile.SubnetID)
	if err != nil {
		return err
	}

	masterSubnet, err := subnet.Get(ctx, &doc.OpenShiftCluster.Properties.ServicePrincipalProfile, doc.OpenShiftCluster.Properties.MasterProfile.SubnetID, i.TenantID)
	if err != nil {
		return err
	}

	_, masterSubnetCIDR, err := net.ParseCIDR(*masterSubnet.AddressPrefix)
	if err != nil {
		return err
	}

	var lbIP net.IP
	{
		_, last := cidr.AddressRange(masterSubnetCIDR)
		lbIP = cidr.Dec(cidr.Dec(last))
	}

	srvRecords := make([]privatedns.SrvRecord, len(machinesMaster.MachineFiles))
	for i := 0; i < len(machinesMaster.MachineFiles); i++ {
		srvRecords[i] = privatedns.SrvRecord{
			Priority: to.Int32Ptr(10),
			Weight:   to.Int32Ptr(10),
			Port:     to.Int32Ptr(2380),
			Target:   to.StringPtr(fmt.Sprintf("etcd-%d.%s", i, installConfig.Config.ObjectMeta.Name+"."+installConfig.Config.BaseDomain)),
		}
	}

	{
		t := &arm.Template{
			Schema:         "https://schema.management.azure.com/schemas/2015-01-01/deploymentTemplate.json#",
			ContentVersion: "1.0.0.0",
			Parameters: map[string]arm.Parameter{
				"sas": {
					Type: "object",
				},
			},
			Resources: []arm.Resource{
				{
					Resource: &authorization.RoleAssignment{
						Name: to.StringPtr("[guid(resourceId('Microsoft.ManagedIdentity/userAssignedIdentities', '" + doc.OpenShiftCluster.Properties.ClusterID + "-identity'), 'contributor')]"),
						Type: to.StringPtr("Microsoft.Authorization/roleAssignments"),
						Properties: &authorization.RoleAssignmentPropertiesWithScope{
							Scope:            to.StringPtr("[resourceGroup().id]"),
							RoleDefinitionID: to.StringPtr("[resourceId('Microsoft.Authorization/roleDefinitions', 'b24988ac-6180-42a0-ab88-20f7382dd24c')]"), // Contributor
							PrincipalID:      to.StringPtr("[reference(resourceId('Microsoft.ManagedIdentity/userAssignedIdentities', '" + doc.OpenShiftCluster.Properties.ClusterID + "-identity'), '2018-11-30').principalId]"),
						},
					},
					APIVersion: apiVersions["authorization"],
				},
				{
					Resource: &privatedns.PrivateZone{
						Name:     to.StringPtr(installConfig.Config.ObjectMeta.Name + "." + installConfig.Config.BaseDomain),
						Type:     to.StringPtr("Microsoft.Network/privateDnsZones"),
						Location: to.StringPtr("global"),
					},
					APIVersion: apiVersions["privatedns"],
				},
				{
					Resource: &privatedns.VirtualNetworkLink{
						VirtualNetworkLinkProperties: &privatedns.VirtualNetworkLinkProperties{
							VirtualNetwork: &privatedns.SubResource{
								ID: to.StringPtr(vnetID),
							},
							RegistrationEnabled: to.BoolPtr(false),
						},
						Name:     to.StringPtr(installConfig.Config.ObjectMeta.Name + "." + installConfig.Config.BaseDomain + "/" + installConfig.Config.ObjectMeta.Name + "-network-link"),
						Type:     to.StringPtr("Microsoft.Network/privateDnsZones/virtualNetworkLinks"),
						Location: to.StringPtr("global"),
					},
					APIVersion: apiVersions["privatedns"],
					DependsOn: []string{
						"Microsoft.Network/privateDnsZones/" + installConfig.Config.ObjectMeta.Name + "." + installConfig.Config.BaseDomain,
					},
				},
				{
					Resource: &privatedns.RecordSet{
						Name: to.StringPtr(installConfig.Config.ObjectMeta.Name + "." + installConfig.Config.BaseDomain + "/api-int"),
						Type: to.StringPtr("Microsoft.Network/privateDnsZones/A"),
						RecordSetProperties: &privatedns.RecordSetProperties{
							TTL: to.Int64Ptr(300),
							ARecords: &[]privatedns.ARecord{
								{
									Ipv4Address: to.StringPtr(lbIP.String()),
								},
							},
						},
					},
					APIVersion: apiVersions["privatedns"],
					DependsOn: []string{
						"Microsoft.Network/privateDnsZones/" + installConfig.Config.ObjectMeta.Name + "." + installConfig.Config.BaseDomain,
					},
				},
				{
					Resource: &privatedns.RecordSet{
						Name: to.StringPtr(installConfig.Config.ObjectMeta.Name + "." + installConfig.Config.BaseDomain + "/api"),
						Type: to.StringPtr("Microsoft.Network/privateDnsZones/A"),
						RecordSetProperties: &privatedns.RecordSetProperties{
							TTL: to.Int64Ptr(300),
							ARecords: &[]privatedns.ARecord{
								{
									Ipv4Address: to.StringPtr(lbIP.String()),
								},
							},
						},
					},
					APIVersion: apiVersions["privatedns"],
					DependsOn: []string{
						"Microsoft.Network/privateDnsZones/" + installConfig.Config.ObjectMeta.Name + "." + installConfig.Config.BaseDomain,
					},
				},
				{
					Resource: &privatedns.RecordSet{
						Name: to.StringPtr(installConfig.Config.ObjectMeta.Name + "." + installConfig.Config.BaseDomain + "/_etcd-server-ssl._tcp"),
						Type: to.StringPtr("Microsoft.Network/privateDnsZones/SRV"),
						RecordSetProperties: &privatedns.RecordSetProperties{
							TTL:        to.Int64Ptr(60),
							SrvRecords: &srvRecords,
						},
					},
					APIVersion: apiVersions["privatedns"],
					DependsOn: []string{
						"Microsoft.Network/privateDnsZones/" + installConfig.Config.ObjectMeta.Name + "." + installConfig.Config.BaseDomain,
					},
				},
				{
					Resource: &privatedns.RecordSet{
						Name: to.StringPtr("[concat('" + installConfig.Config.ObjectMeta.Name + "." + installConfig.Config.BaseDomain + "/etcd-', copyIndex())]"),
						Type: to.StringPtr("Microsoft.Network/privateDnsZones/A"),
						RecordSetProperties: &privatedns.RecordSetProperties{
							TTL: to.Int64Ptr(60),
							ARecords: &[]privatedns.ARecord{
								{
									Ipv4Address: to.StringPtr("[reference(resourceId('Microsoft.Network/networkInterfaces', concat('" + doc.OpenShiftCluster.Properties.ClusterID + "-master', copyIndex(), '-nic')), '2019-07-01').ipConfigurations[0].properties.privateIPAddress]"),
								},
							},
						},
					},
					APIVersion: apiVersions["privatedns"],
					Copy: &arm.Copy{
						Name:  "copy",
						Count: len(machinesMaster.MachineFiles),
					},
					DependsOn: []string{
						"[concat('Microsoft.Network/networkInterfaces/" + doc.OpenShiftCluster.Properties.ClusterID + "-master', copyIndex(), '-nic')]",
						"Microsoft.Network/privateDnsZones/" + installConfig.Config.ObjectMeta.Name + "." + installConfig.Config.BaseDomain,
					},
				},
				{
					// TODO: upstream doesn't appear to wire this in to any vnet - investigate.
					Resource: &network.RouteTable{
						Name:     to.StringPtr(doc.OpenShiftCluster.Properties.ClusterID + "-node-routetable"),
						Type:     to.StringPtr("Microsoft.Network/routeTables"),
						Location: &installConfig.Config.Azure.Region,
					},
					APIVersion: apiVersions["network"],
				},
				{
					Resource: &network.PublicIPAddress{
						Sku: &network.PublicIPAddressSku{
							Name: network.PublicIPAddressSkuNameStandard,
						},
						PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{
							PublicIPAllocationMethod: network.Static,
						},
						Name:     to.StringPtr(doc.OpenShiftCluster.Properties.ClusterID + "-bootstrap-pip"),
						Type:     to.StringPtr("Microsoft.Network/publicIPAddresses"),
						Location: &installConfig.Config.Azure.Region,
					},
					APIVersion: apiVersions["network"],
				},
				{
					Resource: &network.PublicIPAddress{
						Sku: &network.PublicIPAddressSku{
							Name: network.PublicIPAddressSkuNameStandard,
						},
						PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{
							PublicIPAllocationMethod: network.Static,
							DNSSettings: &network.PublicIPAddressDNSSettings{
								DomainNameLabel: &doc.OpenShiftCluster.Properties.ClusterID,
							},
						},
						Name:     to.StringPtr(doc.OpenShiftCluster.Properties.ClusterID + "-pip"),
						Type:     to.StringPtr("Microsoft.Network/publicIPAddresses"),
						Location: &installConfig.Config.Azure.Region,
					},
					APIVersion: apiVersions["network"],
				},
				{
					Resource: &network.LoadBalancer{
						Sku: &network.LoadBalancerSku{
							Name: network.LoadBalancerSkuNameStandard,
						},
						LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
							FrontendIPConfigurations: &[]network.FrontendIPConfiguration{
								{
									FrontendIPConfigurationPropertiesFormat: &network.FrontendIPConfigurationPropertiesFormat{
										PublicIPAddress: &network.PublicIPAddress{
											ID: to.StringPtr("[resourceId('Microsoft.Network/publicIPAddresses', '" + doc.OpenShiftCluster.Properties.ClusterID + "-pip')]"),
										},
									},
									Name: to.StringPtr("public-lb-ip"),
								},
							},
							BackendAddressPools: &[]network.BackendAddressPool{
								{
									Name: to.StringPtr(doc.OpenShiftCluster.Properties.ClusterID + "-public-lb-control-plane"),
								},
							},
							LoadBalancingRules: &[]network.LoadBalancingRule{
								{
									LoadBalancingRulePropertiesFormat: &network.LoadBalancingRulePropertiesFormat{
										FrontendIPConfiguration: &network.SubResource{
											ID: to.StringPtr("[resourceId('Microsoft.Network/loadBalancers/frontendIPConfigurations', '" + doc.OpenShiftCluster.Properties.ClusterID + "-public-lb', 'public-lb-ip')]"),
										},
										BackendAddressPool: &network.SubResource{
											ID: to.StringPtr("[resourceId('Microsoft.Network/loadBalancers/backendAddressPools', '" + doc.OpenShiftCluster.Properties.ClusterID + "-public-lb', '" + doc.OpenShiftCluster.Properties.ClusterID + "-public-lb-control-plane')]"),
										},
										Probe: &network.SubResource{
											ID: to.StringPtr("[resourceId('Microsoft.Network/loadBalancers/probes', '" + doc.OpenShiftCluster.Properties.ClusterID + "-public-lb', 'api-internal-probe')]"),
										},
										Protocol:             network.TransportProtocolTCP,
										LoadDistribution:     network.LoadDistributionDefault,
										FrontendPort:         to.Int32Ptr(6443),
										BackendPort:          to.Int32Ptr(6443),
										IdleTimeoutInMinutes: to.Int32Ptr(30),
									},
									Name: to.StringPtr("api-internal"),
								},
							},
							Probes: &[]network.Probe{
								{
									ProbePropertiesFormat: &network.ProbePropertiesFormat{
										Protocol:          network.ProbeProtocolTCP,
										Port:              to.Int32Ptr(6443),
										IntervalInSeconds: to.Int32Ptr(10),
										NumberOfProbes:    to.Int32Ptr(3),
									},
									Name: to.StringPtr("api-internal-probe"),
									Type: to.StringPtr("Microsoft.Network/loadBalancers/probes"),
								},
							},
						},
						Name:     to.StringPtr(doc.OpenShiftCluster.Properties.ClusterID + "-public-lb"),
						Type:     to.StringPtr("Microsoft.Network/loadBalancers"),
						Location: &installConfig.Config.Azure.Region,
					},
					APIVersion: apiVersions["network"],
					DependsOn: []string{
						"Microsoft.Network/publicIPAddresses/" + doc.OpenShiftCluster.Properties.ClusterID + "-pip",
					},
				},
				{
					Resource: &network.LoadBalancer{
						Sku: &network.LoadBalancerSku{
							Name: network.LoadBalancerSkuNameStandard,
						},
						LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
							FrontendIPConfigurations: &[]network.FrontendIPConfiguration{
								{
									FrontendIPConfigurationPropertiesFormat: &network.FrontendIPConfigurationPropertiesFormat{
										PrivateIPAddress:          to.StringPtr(lbIP.String()),
										PrivateIPAllocationMethod: network.Static,
										Subnet: &network.Subnet{
											ID: to.StringPtr(doc.OpenShiftCluster.Properties.MasterProfile.SubnetID),
										},
									},
									Name: to.StringPtr("internal-lb-ip"),
								},
							},
							BackendAddressPools: &[]network.BackendAddressPool{
								{
									Name: to.StringPtr(doc.OpenShiftCluster.Properties.ClusterID + "-internal-controlplane"),
								},
							},
							LoadBalancingRules: &[]network.LoadBalancingRule{
								{
									LoadBalancingRulePropertiesFormat: &network.LoadBalancingRulePropertiesFormat{
										FrontendIPConfiguration: &network.SubResource{
											ID: to.StringPtr("[resourceId('Microsoft.Network/loadBalancers/frontendIPConfigurations', '" + doc.OpenShiftCluster.Properties.ClusterID + "-internal-lb', 'internal-lb-ip')]"),
										},
										BackendAddressPool: &network.SubResource{
											ID: to.StringPtr("[resourceId('Microsoft.Network/loadBalancers/backendAddressPools', '" + doc.OpenShiftCluster.Properties.ClusterID + "-internal-lb', '" + doc.OpenShiftCluster.Properties.ClusterID + "-internal-controlplane')]"),
										},
										Probe: &network.SubResource{
											ID: to.StringPtr("[resourceId('Microsoft.Network/loadBalancers/probes', '" + doc.OpenShiftCluster.Properties.ClusterID + "-internal-lb', 'api-internal-probe')]"),
										},
										Protocol:             network.TransportProtocolTCP,
										LoadDistribution:     network.LoadDistributionDefault,
										FrontendPort:         to.Int32Ptr(6443),
										BackendPort:          to.Int32Ptr(6443),
										IdleTimeoutInMinutes: to.Int32Ptr(30),
									},
									Name: to.StringPtr("api-internal"),
								},
								{
									LoadBalancingRulePropertiesFormat: &network.LoadBalancingRulePropertiesFormat{
										FrontendIPConfiguration: &network.SubResource{
											ID: to.StringPtr("[resourceId('Microsoft.Network/loadBalancers/frontendIPConfigurations', '" + doc.OpenShiftCluster.Properties.ClusterID + "-internal-lb', 'internal-lb-ip')]"),
										},
										BackendAddressPool: &network.SubResource{
											ID: to.StringPtr("[resourceId('Microsoft.Network/loadBalancers/backendAddressPools', '" + doc.OpenShiftCluster.Properties.ClusterID + "-internal-lb', '" + doc.OpenShiftCluster.Properties.ClusterID + "-internal-controlplane')]"),
										},
										Probe: &network.SubResource{
											ID: to.StringPtr("[resourceId('Microsoft.Network/loadBalancers/probes', '" + doc.OpenShiftCluster.Properties.ClusterID + "-internal-lb', 'sint-probe')]"),
										},
										Protocol:             network.TransportProtocolTCP,
										LoadDistribution:     network.LoadDistributionDefault,
										FrontendPort:         to.Int32Ptr(22623),
										BackendPort:          to.Int32Ptr(22623),
										IdleTimeoutInMinutes: to.Int32Ptr(30),
									},
									Name: to.StringPtr("sint"),
								},
							},
							Probes: &[]network.Probe{
								{
									ProbePropertiesFormat: &network.ProbePropertiesFormat{
										Protocol:          network.ProbeProtocolTCP,
										Port:              to.Int32Ptr(6443),
										IntervalInSeconds: to.Int32Ptr(10),
										NumberOfProbes:    to.Int32Ptr(3),
									},
									Name: to.StringPtr("api-internal-probe"),
								},
								{
									ProbePropertiesFormat: &network.ProbePropertiesFormat{
										Protocol:          network.ProbeProtocolTCP,
										Port:              to.Int32Ptr(22623),
										IntervalInSeconds: to.Int32Ptr(10),
										NumberOfProbes:    to.Int32Ptr(3),
									},
									Name: to.StringPtr("sint-probe"),
								},
							},
						},
						Name:     to.StringPtr(doc.OpenShiftCluster.Properties.ClusterID + "-internal-lb"),
						Type:     to.StringPtr("Microsoft.Network/loadBalancers"),
						Location: &installConfig.Config.Azure.Region,
					},
					APIVersion: apiVersions["network"],
					DependsOn: []string{
						"Microsoft.Network/privateDnsZones/" + installConfig.Config.ObjectMeta.Name + "." + installConfig.Config.BaseDomain,
					},
				},
				{
					Resource: &network.Interface{
						InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
							IPConfigurations: &[]network.InterfaceIPConfiguration{
								{
									InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
										LoadBalancerBackendAddressPools: &[]network.BackendAddressPool{
											{
												ID: to.StringPtr("[resourceId('Microsoft.Network/loadBalancers/backendAddressPools', '" + doc.OpenShiftCluster.Properties.ClusterID + "-public-lb', '" + doc.OpenShiftCluster.Properties.ClusterID + "-public-lb-control-plane')]"),
											},
											{
												ID: to.StringPtr("[resourceId('Microsoft.Network/loadBalancers/backendAddressPools', '" + doc.OpenShiftCluster.Properties.ClusterID + "-internal-lb', '" + doc.OpenShiftCluster.Properties.ClusterID + "-internal-controlplane')]"),
											},
										},
										Subnet: &network.Subnet{
											ID: to.StringPtr(doc.OpenShiftCluster.Properties.MasterProfile.SubnetID),
										},
										PublicIPAddress: &network.PublicIPAddress{
											ID: to.StringPtr("[resourceId('Microsoft.Network/publicIPAddresses', '" + doc.OpenShiftCluster.Properties.ClusterID + "-bootstrap-pip')]"),
										},
									},
									Name: to.StringPtr("bootstrap-nic-ip"),
								},
							},
						},
						Name:     to.StringPtr(doc.OpenShiftCluster.Properties.ClusterID + "-bootstrap-nic"),
						Type:     to.StringPtr("Microsoft.Network/networkInterfaces"),
						Location: &installConfig.Config.Azure.Region,
					},
					APIVersion: apiVersions["network"],
					DependsOn: []string{
						"Microsoft.Network/loadBalancers/" + doc.OpenShiftCluster.Properties.ClusterID + "-internal-lb",
						"Microsoft.Network/loadBalancers/" + doc.OpenShiftCluster.Properties.ClusterID + "-public-lb",
						"Microsoft.Network/publicIPAddresses/" + doc.OpenShiftCluster.Properties.ClusterID + "-bootstrap-pip",
					},
				},
				{
					Resource: &network.Interface{
						InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
							IPConfigurations: &[]network.InterfaceIPConfiguration{
								{
									InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
										LoadBalancerBackendAddressPools: &[]network.BackendAddressPool{
											{
												ID: to.StringPtr("[resourceId('Microsoft.Network/loadBalancers/backendAddressPools', '" + doc.OpenShiftCluster.Properties.ClusterID + "-public-lb', '" + doc.OpenShiftCluster.Properties.ClusterID + "-public-lb-control-plane')]"),
											},
											{
												ID: to.StringPtr("[resourceId('Microsoft.Network/loadBalancers/backendAddressPools', '" + doc.OpenShiftCluster.Properties.ClusterID + "-internal-lb', '" + doc.OpenShiftCluster.Properties.ClusterID + "-internal-controlplane')]"),
											},
										},
										Subnet: &network.Subnet{
											ID: to.StringPtr(doc.OpenShiftCluster.Properties.MasterProfile.SubnetID),
										},
									},
									Name: to.StringPtr("pipConfig"),
								},
							},
						},
						Name:     to.StringPtr("[concat('" + doc.OpenShiftCluster.Properties.ClusterID + "-master', copyIndex(), '-nic')]"),
						Type:     to.StringPtr("Microsoft.Network/networkInterfaces"),
						Location: &installConfig.Config.Azure.Region,
					},
					APIVersion: apiVersions["network"],
					Copy: &arm.Copy{
						Name:  "copy",
						Count: len(machinesMaster.MachineFiles),
					},
					DependsOn: []string{
						"Microsoft.Network/loadBalancers/" + doc.OpenShiftCluster.Properties.ClusterID + "-internal-lb",
						"Microsoft.Network/loadBalancers/" + doc.OpenShiftCluster.Properties.ClusterID + "-public-lb",
					},
				},
				{
					Resource: &compute.Image{
						ImageProperties: &compute.ImageProperties{
							StorageProfile: &compute.ImageStorageProfile{
								OsDisk: &compute.ImageOSDisk{
									OsType:  compute.Linux,
									BlobURI: to.StringPtr("https://cluster" + doc.OpenShiftCluster.Properties.StorageSuffix + ".blob.core.windows.net/vhd/rhcos" + doc.OpenShiftCluster.Properties.StorageSuffix + ".vhd"),
								},
							},
							HyperVGeneration: compute.HyperVGenerationTypesV1,
						},
						Name:     &doc.OpenShiftCluster.Properties.ClusterID,
						Type:     to.StringPtr("Microsoft.Compute/images"),
						Location: &installConfig.Config.Azure.Region,
					},
					APIVersion: apiVersions["compute"],
				},
				{
					Resource: &compute.VirtualMachine{
						VirtualMachineProperties: &compute.VirtualMachineProperties{
							HardwareProfile: &compute.HardwareProfile{
								VMSize: compute.VirtualMachineSizeTypesStandardD4sV3,
							},
							StorageProfile: &compute.StorageProfile{
								ImageReference: &compute.ImageReference{
									ID: to.StringPtr("[resourceId('Microsoft.Compute/images', '" + doc.OpenShiftCluster.Properties.ClusterID + "')]"),
								},
								OsDisk: &compute.OSDisk{
									Name:         to.StringPtr(doc.OpenShiftCluster.Properties.ClusterID + "-bootstrap_OSDisk"),
									Caching:      compute.CachingTypesReadWrite,
									CreateOption: compute.DiskCreateOptionTypesFromImage,
									DiskSizeGB:   to.Int32Ptr(100),
									ManagedDisk: &compute.ManagedDiskParameters{
										StorageAccountType: compute.StorageAccountTypesPremiumLRS,
									},
								},
							},
							OsProfile: &compute.OSProfile{
								ComputerName:  to.StringPtr(doc.OpenShiftCluster.Properties.ClusterID + "-bootstrap-vm"),
								AdminUsername: to.StringPtr("core"),
								AdminPassword: to.StringPtr("NotActuallyApplied!"),
								CustomData:    to.StringPtr(`[base64(concat('{"ignition":{"version":"2.2.0","config":{"replace":{"source":"https://cluster` + doc.OpenShiftCluster.Properties.StorageSuffix + `.blob.core.windows.net/ignition/bootstrap.ign?', listAccountSas(resourceId('Microsoft.Storage/storageAccounts', 'cluster` + doc.OpenShiftCluster.Properties.StorageSuffix + `'), '2019-04-01', parameters('sas')).accountSasToken, '"}}}}'))]`),
								LinuxConfiguration: &compute.LinuxConfiguration{
									DisablePasswordAuthentication: to.BoolPtr(false),
								},
							},
							NetworkProfile: &compute.NetworkProfile{
								NetworkInterfaces: &[]compute.NetworkInterfaceReference{
									{
										ID: to.StringPtr("[resourceId('Microsoft.Network/networkInterfaces', '" + doc.OpenShiftCluster.Properties.ClusterID + "-bootstrap-nic')]"),
									},
								},
							},
							DiagnosticsProfile: &compute.DiagnosticsProfile{
								BootDiagnostics: &compute.BootDiagnostics{
									Enabled:    to.BoolPtr(true),
									StorageURI: to.StringPtr("https://cluster" + doc.OpenShiftCluster.Properties.StorageSuffix + ".blob.core.windows.net/"),
								},
							},
						},
						Identity: &compute.VirtualMachineIdentity{
							Type: compute.ResourceIdentityTypeUserAssigned,
							UserAssignedIdentities: map[string]*compute.VirtualMachineIdentityUserAssignedIdentitiesValue{
								"[resourceId('Microsoft.ManagedIdentity/userAssignedIdentities', '" + doc.OpenShiftCluster.Properties.ClusterID + "-identity')]": &compute.VirtualMachineIdentityUserAssignedIdentitiesValue{},
							},
						},
						Name:     to.StringPtr(doc.OpenShiftCluster.Properties.ClusterID + "-bootstrap"),
						Type:     to.StringPtr("Microsoft.Compute/virtualMachines"),
						Location: &installConfig.Config.Azure.Region,
					},
					APIVersion: apiVersions["compute"],
					DependsOn: []string{
						"Microsoft.Compute/images/" + doc.OpenShiftCluster.Properties.ClusterID,
						"Microsoft.Network/networkInterfaces/" + doc.OpenShiftCluster.Properties.ClusterID + "-bootstrap-nic",
						"Microsoft.Network/privateDnsZones/" + installConfig.Config.ObjectMeta.Name + "." + installConfig.Config.BaseDomain + "/virtualNetworkLinks/" + installConfig.Config.ObjectMeta.Name + "-network-link",
					},
				},
				{
					Resource: &compute.VirtualMachine{
						VirtualMachineProperties: &compute.VirtualMachineProperties{
							HardwareProfile: &compute.HardwareProfile{
								VMSize: compute.VirtualMachineSizeTypes(installConfig.Config.ControlPlane.Platform.Azure.InstanceType),
							},
							StorageProfile: &compute.StorageProfile{
								ImageReference: &compute.ImageReference{
									ID: to.StringPtr("[resourceId('Microsoft.Compute/images', '" + doc.OpenShiftCluster.Properties.ClusterID + "')]"),
								},
								OsDisk: &compute.OSDisk{
									Name:         to.StringPtr("[concat('" + doc.OpenShiftCluster.Properties.ClusterID + "-master-', copyIndex(), '_OSDisk')]"),
									Caching:      compute.CachingTypesReadOnly,
									CreateOption: compute.DiskCreateOptionTypesFromImage,
									DiskSizeGB:   &installConfig.Config.ControlPlane.Platform.Azure.OSDisk.DiskSizeGB,
									ManagedDisk: &compute.ManagedDiskParameters{
										StorageAccountType: compute.StorageAccountTypesPremiumLRS,
									},
								},
							},
							OsProfile: &compute.OSProfile{
								ComputerName:  to.StringPtr("[concat('" + doc.OpenShiftCluster.Properties.ClusterID + "-master-', copyIndex())]"),
								AdminUsername: to.StringPtr("core"),
								AdminPassword: to.StringPtr("NotActuallyApplied!"),
								CustomData:    to.StringPtr(base64.StdEncoding.EncodeToString(machineMaster.File.Data)),
								LinuxConfiguration: &compute.LinuxConfiguration{
									DisablePasswordAuthentication: to.BoolPtr(false),
								},
							},
							NetworkProfile: &compute.NetworkProfile{
								NetworkInterfaces: &[]compute.NetworkInterfaceReference{
									{
										ID: to.StringPtr("[resourceId('Microsoft.Network/networkInterfaces', concat('" + doc.OpenShiftCluster.Properties.ClusterID + "-master', copyIndex(), '-nic'))]"),
									},
								},
							},
							DiagnosticsProfile: &compute.DiagnosticsProfile{
								BootDiagnostics: &compute.BootDiagnostics{
									Enabled:    to.BoolPtr(true),
									StorageURI: to.StringPtr("https://cluster" + doc.OpenShiftCluster.Properties.StorageSuffix + ".blob.core.windows.net/"),
								},
							},
						},
						Identity: &compute.VirtualMachineIdentity{
							Type: compute.ResourceIdentityTypeUserAssigned,
							UserAssignedIdentities: map[string]*compute.VirtualMachineIdentityUserAssignedIdentitiesValue{
								"[resourceId('Microsoft.ManagedIdentity/userAssignedIdentities', '" + doc.OpenShiftCluster.Properties.ClusterID + "-identity')]": &compute.VirtualMachineIdentityUserAssignedIdentitiesValue{},
							},
						},
						Zones: &[]string{
							"[copyIndex(1)]",
						},
						Name:     to.StringPtr("[concat('" + doc.OpenShiftCluster.Properties.ClusterID + "-master-', copyIndex())]"),
						Type:     to.StringPtr("Microsoft.Compute/virtualMachines"),
						Location: &installConfig.Config.Azure.Region,
					},
					APIVersion: apiVersions["compute"],
					Copy: &arm.Copy{
						Name:  "copy",
						Count: len(machinesMaster.MachineFiles),
					},
					DependsOn: []string{
						"Microsoft.Compute/images/" + doc.OpenShiftCluster.Properties.ClusterID,
						"[concat('Microsoft.Network/networkInterfaces/" + doc.OpenShiftCluster.Properties.ClusterID + "-master', copyIndex(), '-nic')]",
						"Microsoft.Network/privateDnsZones/" + installConfig.Config.ObjectMeta.Name + "." + installConfig.Config.BaseDomain + "/virtualNetworkLinks/" + installConfig.Config.ObjectMeta.Name + "-network-link",
					},
				},
			},
		}

		i.log.Print("deploying resources template")
		future, err := i.deployments.CreateOrUpdate(ctx, doc.OpenShiftCluster.Properties.ResourceGroup, "azuredeploy", resources.Deployment{
			Properties: &resources.DeploymentProperties{
				Template: t,
				Parameters: map[string]interface{}{
					"sas": map[string]interface{}{
						"value": map[string]interface{}{
							"signedStart":         doc.OpenShiftCluster.Properties.Installation.Now.UTC().Format(time.RFC3339),
							"signedExpiry":        doc.OpenShiftCluster.Properties.Installation.Now.Add(24 * time.Hour).Format(time.RFC3339),
							"signedPermission":    "rl",
							"signedResourceTypes": "o",
							"signedServices":      "b",
							"signedProtocol":      "https",
						},
					},
				},
				Mode: resources.Incremental,
			},
		})
		if err != nil {
			return err
		}

		i.log.Print("waiting for resources template deployment")
		err = future.WaitForCompletionRef(ctx, i.deployments.Client)
		if err != nil {
			return err
		}
	}

	{
		_, err = i.recordsets.CreateOrUpdate(ctx, installConfig.Config.Azure.BaseDomainResourceGroupName, installConfig.Config.BaseDomain, "api."+installConfig.Config.ObjectMeta.Name, dns.CNAME, dns.RecordSet{
			RecordSetProperties: &dns.RecordSetProperties{
				TTL: to.Int64Ptr(300),
				CnameRecord: &dns.CnameRecord{
					Cname: to.StringPtr(doc.OpenShiftCluster.Properties.ClusterID + "." + installConfig.Config.Azure.Region + ".cloudapp.azure.com"),
				},
			},
		}, "", "")
		if err != nil {
			return err
		}
	}

	{
		restConfig, err := restconfig.RestConfig(doc.OpenShiftCluster.Properties.AdminKubeconfig)
		if err != nil {
			return err
		}

		cli, err := kubernetes.NewForConfig(restConfig)
		if err != nil {
			return err
		}

		i.log.Print("waiting for bootstrap configmap")
		now := time.Now()
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()
		for {
			cm, err := cli.CoreV1().ConfigMaps("kube-system").Get("bootstrap", metav1.GetOptions{})
			if err == nil && cm.Data["status"] == "complete" {
				break
			}

			if time.Now().Sub(now) > 30*time.Minute {
				return fmt.Errorf("timed out waiting for bootstrap configmap. Last error: %v", err)
			}

			<-t.C
		}
	}

	return nil
}
