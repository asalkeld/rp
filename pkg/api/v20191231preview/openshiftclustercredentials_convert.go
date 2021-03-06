package v20191231preview

import (
	"github.com/jim-minter/rp/pkg/api"
)

// OpenShiftClusterCredentialsToExternal returns a new external representation
// of the internal object, reading from the subset of the internal object's
// fields that appear in the external representation.  ToExternal does not
// modify its argument; there is no pointer aliasing between the passed and
// returned objects.
func OpenShiftClusterCredentialsToExternal(oc *api.OpenShiftCluster) *OpenShiftClusterCredentials {
	out := &OpenShiftClusterCredentials{
		KubeadminPassword: oc.Properties.KubeadminPassword,
	}

	return out
}

// ToInternal overwrites in place a pre-existing internal object, setting (only)
// all mapped fields from the external representation.  ToInternal modifies its
// argument; there is no pointer aliasing between the passed and returned
// objects.
func (occ *OpenShiftClusterCredentials) ToInternal(out *api.OpenShiftCluster) {
	out.Properties.KubeadminPassword = occ.KubeadminPassword
}
