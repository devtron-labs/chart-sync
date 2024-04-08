//go:build wireinject
// +build wireinject

package main

import (
	"github.com/devtron-labs/chart-sync/internals"
	"github.com/devtron-labs/chart-sync/internals/logger"
	"github.com/devtron-labs/chart-sync/internals/sql"
	"github.com/devtron-labs/chart-sync/pkg"
	"github.com/devtron-labs/chart-sync/pkg/registry"
	"github.com/google/wire"
)

func InitializeApp() (*App, error) {
	wire.Build(
		NewApp,
		logger.NewSugardLogger,
		sql.GetConfig,
		internals.ParseConfiguration,
		sql.NewDbConnection,
		sql.NewDockerArtifactStoreRepositoryImpl,
		wire.Bind(new(sql.DockerArtifactStoreRepository), new(*sql.DockerArtifactStoreRepositoryImpl)),
		sql.NewOCIRegistryConfigRepositoryImpl,
		wire.Bind(new(sql.OCIRegistryConfigRepository), new(*sql.OCIRegistryConfigRepositoryImpl)),
		sql.NewChartRepoRepositoryImpl,
		wire.Bind(new(sql.ChartRepoRepository), new(*sql.ChartRepoRepositoryImpl)),
		sql.NewAppStoreRepositoryImpl,
		wire.Bind(new(sql.AppStoreRepository), new(*sql.AppStoreRepositoryImpl)),
		sql.NewAppStoreApplicationVersionRepositoryImpl,
		wire.Bind(new(sql.AppStoreApplicationVersionRepository), new(*sql.AppStoreApplicationVersionRepositoryImpl)),
		pkg.NewHelmRepoManagerImpl,
		wire.Bind(new(pkg.HelmRepoManager), new(*pkg.HelmRepoManagerImpl)),
		pkg.NewSyncServiceImpl,
		wire.Bind(new(pkg.SyncService), new(*pkg.SyncServiceImpl)),
		registry.NewClientGetterImpl,
		wire.Bind(new(registry.ClientGetter), new(*registry.ClientGetterImpl)),
		sql.NewRemoteConnectionRepositoryImpl,
		wire.Bind(new(sql.RemoteConnectionRepository), new(*sql.RemoteConnectionRepositoryImpl)),
	)
	return &App{}, nil
}
