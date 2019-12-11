package resources

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-05-01/resources"
)

// DeploymentsClientAddons contains addons for DeploymentsClient
type DeploymentsClientAddons interface {
	CreateOrUpdateAndWait(ctx context.Context, resourceGroupName string, deploymentName string, parameters resources.Deployment) error
	DeleteAndWait(ctx context.Context, resourceGroupName string, deploymentName string) error
}

func (c *deploymentsClient) CreateOrUpdateAndWait(ctx context.Context, resourceGroupName string, deploymentName string, parameters resources.Deployment) error {
	future, err := c.CreateOrUpdate(ctx, resourceGroupName, deploymentName, parameters)
	if err != nil {
		return err
	}

	return future.WaitForCompletionRef(ctx, c.DeploymentsClient.Client)
}

func (c *deploymentsClient) DeleteAndWait(ctx context.Context, resourceGroupName string, deploymentName string) error {
	future, err := c.Delete(ctx, resourceGroupName, deploymentName)
	if err != nil {
		return err
	}

	return future.WaitForCompletionRef(ctx, c.DeploymentsClient.Client)
}
