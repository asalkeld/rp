package env

//go:generate go run ../../vendor/github.com/golang/mock/mockgen -destination=../util/mocks/mock_$GOPACKAGE/$GOPACKAGE.go github.com/jim-minter/rp/pkg/$GOPACKAGE Interface
//go:generate go run ../../vendor/golang.org/x/tools/cmd/goimports -local=github.com/jim-minter/rp -e -w ../util/mocks/mock_$GOPACKAGE/$GOPACKAGE.go

import (
	"context"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/Azure/go-autorest/autorest"
	"github.com/sirupsen/logrus"

	"github.com/jim-minter/rp/pkg/env/dev"
	"github.com/jim-minter/rp/pkg/env/prod"
	"github.com/jim-minter/rp/pkg/env/shared/dns"
)

type Interface interface {
	CosmosDB(context.Context) (string, string, error)
	DNS() dns.Manager
	FPAuthorizer(context.Context, string) (autorest.Authorizer, error)
	IsReady() bool
	ListenTLS(context.Context) (net.Listener, error)
	Authenticated(http.Handler) http.Handler
	Location() string
	ResourceGroup() string
}

func NewEnv(ctx context.Context, log *logrus.Entry) (Interface, error) {
	if strings.ToLower(os.Getenv("RP_MODE")) == "development" {
		log.Warn("running in development mode")
		return dev.New(ctx, log)
	}
	return prod.New(ctx, log)
}
