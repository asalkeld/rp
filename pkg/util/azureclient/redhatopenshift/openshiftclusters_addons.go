package redhatopenshift

import (
	"context"

	"github.com/jim-minter/rp/pkg/client/services/preview/redhatopenshift/mgmt/2019-12-31-preview/redhatopenshift"
)

// OpenShiftClustersClientAddons contains addons for OpenShiftClustersClient
type OpenShiftClustersClientAddons interface {
	CreateOrUpdateAndWait(ctx context.Context, resourceGroupName string, resourceName string, parameters redhatopenshift.OpenShiftCluster) error
	UpdateAndWait(ctx context.Context, resourceGroupName string, resourceName string, parameters redhatopenshift.OpenShiftCluster) error
	DeleteAndWait(ctx context.Context, resourceGroupName string, deploymentName string) error
}

func (c *openShiftClustersClient) CreateOrUpdateAndWait(ctx context.Context, resourceGroupName string, resourceName string, parameters redhatopenshift.OpenShiftCluster) error {
	future, err := c.CreateOrUpdate(ctx, resourceGroupName, resourceName, parameters)
	if err != nil {
		return err
	}

	return future.WaitForCompletionRef(ctx, c.OpenShiftClustersClient.Client)
}

func (c *openShiftClustersClient) UpdateAndWait(ctx context.Context, resourceGroupName string, resourceName string, parameters redhatopenshift.OpenShiftCluster) error {
	future, err := c.Update(ctx, resourceGroupName, resourceName, parameters)
	if err != nil {
		return err
	}

	return future.WaitForCompletionRef(ctx, c.OpenShiftClustersClient.Client)
}

func (c *openShiftClustersClient) DeleteAndWait(ctx context.Context, resourceGroupName string, resourceName string) error {
	future, err := c.Delete(ctx, resourceGroupName, resourceName)
	if err != nil {
		return err
	}

	return future.WaitForCompletionRef(ctx, c.OpenShiftClustersClient.Client)
}
