package openshiftcluster

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"

	"github.com/jim-minter/rp/pkg/api"
	"github.com/jim-minter/rp/pkg/util/mocks"
	"github.com/jim-minter/rp/pkg/util/mocks/mock_azureclient/mock_resources"
	"github.com/jim-minter/rp/pkg/util/mocks/mock_database"
	"github.com/jim-minter/rp/pkg/util/mocks/mock_env"
	"github.com/jim-minter/rp/pkg/util/mocks/mock_env/shared/mock_dns"
	"github.com/jim-minter/rp/pkg/util/mocks/mock_install"
	"github.com/jim-minter/rp/pkg/util/mocks/mock_subnet"
)

func TestManagerCreate(t *testing.T) {
	ctx := context.TODO()
	gmc := gomock.NewController(t)
	defer gmc.Finish()
	logger := logrus.NewEntry(logrus.StandardLogger())

	doc := &api.OpenShiftClusterDocument{
		Key:              "foo",
		OpenShiftCluster: mocks.MockOpenShiftCluster(),
	}

	mDB := mock_database.NewMockOpenShiftClusters(gmc)
	mDB.EXPECT().Patch(gomock.Any(), gomock.Any()).Return(doc, nil)

	mDNS := mock_dns.NewMockManager(gmc)
	mDNS.EXPECT().Domain().Return("domain")

	mEnv := mock_env.NewMockInterface(gmc)
	mEnv.EXPECT().DNS().Return(mDNS)
	mEnv.EXPECT().ResourceGroup().Return("envrg")
	mEnv.EXPECT().PullSecret().Return(string(mocks.DummyImagePullSecret("registry.redhat.io")))

	mInstall := mock_install.NewMockInterface(gmc)
	mInstall.EXPECT().Install(ctx, doc, gomock.Any(), gomock.Any()).Return(nil)
	m := &Manager{
		log:          logger,
		doc:          doc,
		env:          mEnv,
		db:           mDB,
		fpAuthorizer: nil,
		installer:    mInstall,
		groups:       mock_resources.NewMockGroupsClient(gmc),
		subnets:      mock_subnet.NewMockManager(gmc),
	}

	if err := m.Create(ctx); err != nil {
		t.Errorf("Manager.Create() error = %v", err)
	}
}
