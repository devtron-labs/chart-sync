package registry

import (
	"github.com/devtron-labs/chart-sync/internals/sql"
	"helm.sh/helm/v3/pkg/registry"
)

type ClientGetter interface {
	GetRegistryClient(store *sql.DockerArtifactStore) (*registry.Client, error)
	GetRegistryHostURl(store *sql.DockerArtifactStore) (string, error)
}

type ClientGetterImpl struct {
}

func NewClientGetterImpl() *ClientGetterImpl {
	return &ClientGetterImpl{}
}

func (c *ClientGetterImpl) GetRegistryClient(store *sql.DockerArtifactStore) (*registry.Client, error) {
	registryClient, err := registry.NewClient()
	if err != nil {
		return nil, err
	}
	return registryClient, nil
}

func (c *ClientGetterImpl) GetRegistryHostURl(store *sql.DockerArtifactStore) (string, error) {
	//for enterprise -  registry url will be modified if server connection method is ss
	return store.RegistryURL, nil
}
