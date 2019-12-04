package dns

//go:generate go run ../../../../vendor/github.com/golang/mock/mockgen -destination=../../../util/mocks/mock_env/shared/mock_$GOPACKAGE/$GOPACKAGE.go github.com/jim-minter/rp/pkg/env/shared/$GOPACKAGE Manager
//go:generate go run ../../../../vendor/golang.org/x/tools/cmd/goimports -local=github.com/jim-minter/rp -e -w ../../../util/mocks/mock_env/shared/mock_$GOPACKAGE/$GOPACKAGE.go

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/services/dns/mgmt/2018-05-01/dns"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"

	"github.com/jim-minter/rp/pkg/api"
)

type Manager interface {
	Domain() string
	CreateOrUpdate(context.Context, *api.OpenShiftCluster) error
	Delete(context.Context, *api.OpenShiftCluster) error
}

type manager struct {
	recordsets dns.RecordSetsClient
	zones      dns.ZonesClient

	resourceGroup string
	domain        string
}

func NewManager(ctx context.Context, subscriptionID string, rpAuthorizer autorest.Authorizer, resourceGroup string) (Manager, error) {
	m := &manager{
		recordsets: dns.NewRecordSetsClient(subscriptionID),
		zones:      dns.NewZonesClient(subscriptionID),

		resourceGroup: resourceGroup,
	}

	m.recordsets.Authorizer = rpAuthorizer
	m.zones.Authorizer = rpAuthorizer

	page, err := m.zones.ListByResourceGroup(ctx, m.resourceGroup, nil)
	if err != nil {
		return nil, err
	}

	zones := page.Values()
	if len(zones) != 1 {
		return nil, fmt.Errorf("found at least %d zones, expected 1", len(zones))
	}

	m.domain = *zones[0].Name

	return m, nil
}

func (m *manager) Domain() string {
	return m.domain
}

func (m *manager) CreateOrUpdate(ctx context.Context, oc *api.OpenShiftCluster) error {
	_, err := m.recordsets.CreateOrUpdate(ctx, m.resourceGroup, m.domain, "api."+oc.Properties.DomainName, dns.CNAME, dns.RecordSet{
		RecordSetProperties: &dns.RecordSetProperties{
			TTL: to.Int64Ptr(300),
			CnameRecord: &dns.CnameRecord{
				Cname: to.StringPtr(oc.Properties.DomainName + "." + oc.Location + ".cloudapp.azure.com"),
			},
		},
	}, "", "")

	return err
}

func (m *manager) Delete(ctx context.Context, oc *api.OpenShiftCluster) error {
	_, err := m.recordsets.Delete(ctx, m.resourceGroup, m.domain, "api."+oc.Properties.DomainName, dns.CNAME, "")

	return err
}
