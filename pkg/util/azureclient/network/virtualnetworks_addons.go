package network

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-07-01/network"
)

// VirtualNetworksClientAddons contains addons for VirtualNetworksClient
type VirtualNetworksClientAddons interface {
	CreateOrUpdateAndWait(ctx context.Context, resourceGroupName string, virtualNetworkName string, parameters network.VirtualNetwork) error
	DeleteAndWait(ctx context.Context, resourceGroupName string, publicIPAddressName string) (err error)
}

func (c *virtualNetworksClient) CreateOrUpdateAndWait(ctx context.Context, resourceGroupName string, virtualNetworkName string, parameters network.VirtualNetwork) error {
	future, err := c.CreateOrUpdate(ctx, resourceGroupName, virtualNetworkName, parameters)
	if err != nil {
		return err
	}

	return future.WaitForCompletionRef(ctx, c.VirtualNetworksClient.Client)

}

func (c *virtualNetworksClient) DeleteAndWait(ctx context.Context, resourceGroupName string, publicIPAddressName string) error {
	future, err := c.Delete(ctx, resourceGroupName, publicIPAddressName)
	if err != nil {
		return err
	}

	return future.WaitForCompletionRef(ctx, c.VirtualNetworksClient.Client)
}
