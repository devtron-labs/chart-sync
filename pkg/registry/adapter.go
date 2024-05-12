package registry

import (
	"github.com/devtron-labs/chart-sync/internals/sql"
	"github.com/devtron-labs/common-lib/helmLib/registry"
)

func ConvertToRegistryConfig(store *sql.DockerArtifactStore) *registry.Configuration {
	return &registry.Configuration{
		RegistryId:             store.Id,
		RegistryUrl:            store.RegistryURL,
		Username:               store.Username,
		Password:               store.Password,
		AwsAccessKey:           store.AWSAccessKeyId,
		AwsSecretKey:           store.AWSSecretAccessKey,
		AwsRegion:              store.AWSRegion,
		RegistryConnectionType: store.Connection,
		RegistryCAFilePath:     store.Cert,
		RegistryType:           string(store.RegistryType),
		IsPublicRegistry:       store.OCIRegistryConfig[0].IsPublic,
	}
}
