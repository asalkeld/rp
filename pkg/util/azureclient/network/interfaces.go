package network

//go:generate go run ../../../../vendor/github.com/golang/mock/mockgen -destination=../../../util/mocks/mock_azureclient/mock_$GOPACKAGE/$GOPACKAGE.go github.com/jim-minter/rp/pkg/util/azureclient/$GOPACKAGE InterfacesClient,PublicIPAddressesClient,VirtualNetworksClient
//go:generate go run ../../../../vendor/golang.org/x/tools/cmd/goimports -local=github.com/jim-minter/rp -e -w ../../../util/mocks/mock_azureclient/mock_$GOPACKAGE/$GOPACKAGE.go

import (
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-07-01/network"
	"github.com/Azure/go-autorest/autorest"
)

// InterfacesClient is a minimal interface for azure InterfacesClient
type InterfacesClient interface {
	InterfacesClientAddons
}

type interfacesClient struct {
	network.InterfacesClient
}

var _ InterfacesClient = &interfacesClient{}

// NewInterfacesClient creates a new InterfacesClient
func NewInterfacesClient(subscriptionID string, authorizer autorest.Authorizer) InterfacesClient {
	client := network.NewInterfacesClient(subscriptionID)
	client.Authorizer = authorizer

	return &interfacesClient{
		InterfacesClient: client,
	}
}
