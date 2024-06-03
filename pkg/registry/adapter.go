package registry

import (
	"github.com/devtron-labs/chart-sync/internals/sql"
	"github.com/devtron-labs/common-lib/helmLib/registry"
	"github.com/devtron-labs/common-lib/utils/remoteConnection/bean"
)

func NewToRegistryConfig(store *sql.DockerArtifactStore) (*registry.Configuration, error) {
	remoteConnectionConfig := &bean.RemoteConnectionConfigBean{}
	if store.RemoteConnectionConfig != nil && store.RemoteConnectionConfigId > 0 {
		remoteConnectionConfig.ConnectionMethod = bean.RemoteConnectionMethod(store.RemoteConnectionConfig.ConnectionMethod)
		switch remoteConnectionConfig.ConnectionMethod {
		case bean.RemoteConnectionMethodProxy:
			remoteConnectionConfig.ProxyConfig = &bean.ProxyConfig{
				ProxyUrl: store.RemoteConnectionConfig.ProxyUrl,
			}
		case bean.RemoteConnectionMethodSSH:
			remoteConnectionConfig.SSHTunnelConfig = &bean.SSHTunnelConfig{
				SSHServerAddress: store.RemoteConnectionConfig.SSHServerAddress,
				SSHUsername:      store.RemoteConnectionConfig.SSHUsername,
				SSHPassword:      store.RemoteConnectionConfig.SSHPassword,
				SSHAuthKey:       store.RemoteConnectionConfig.SSHAuthKey,
			}
		}
	}
	var certificatePath string
	var err error
	if store.Connection == registry.SECURE_WITH_CERT {
		certificatePath, err = registry.CreateCertificateFile(store.Id, store.Cert)
		if err != nil {
			return nil, err
		}
	}
	return &registry.Configuration{
		RegistryId:                store.Id,
		RegistryUrl:               store.RegistryURL,
		Username:                  store.Username,
		Password:                  store.Password,
		AwsAccessKey:              store.AWSAccessKeyId,
		AwsSecretKey:              store.AWSSecretAccessKey,
		AwsRegion:                 store.AWSRegion,
		RegistryConnectionType:    store.Connection,
		RegistryCertificateString: store.Cert,
		RegistryCAFilePath:        certificatePath,
		RegistryType:              string(store.RegistryType),
		IsPublicRegistry:          store.OCIRegistryConfig[0].IsPublic,
		RemoteConnectionConfig:    remoteConnectionConfig,
	}, nil
}
