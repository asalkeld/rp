package env

import (
	"context"
	"net"
	"net/http"

	"github.com/Azure/go-autorest/autorest"
	"github.com/sirupsen/logrus"

	"github.com/jim-minter/rp/pkg/env/dev"
	"github.com/jim-minter/rp/pkg/env/prod"
)

type Interface interface {
	CosmosDB(ctx context.Context) (string, string, error)
	DNS(ctx context.Context) (string, error)
	FirstPartyAuthorizer(ctx context.Context) (autorest.Authorizer, error)
	IsReady() bool
	ListenTLS(ctx context.Context) (net.Listener, error)
	Authenticated(h http.Handler) http.Handler
}

func NewEnv(ctx context.Context, log *logrus.Entry) (Interface, error) {
	if dev.IsDevelopment() {
		log.Warn("running in development mode")
		return dev.New(ctx, log)
	}
	return prod.New(ctx, log)
}
