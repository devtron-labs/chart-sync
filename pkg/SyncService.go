package pkg

import (
	"encoding/json"
	"fmt"
	"github.com/devtron-labs/chart-sync/internal"
	"github.com/devtron-labs/chart-sync/internal/sql"
	"github.com/devtron-labs/chart-sync/util"
	"github.com/ghodss/yaml"
	"github.com/go-pg/pg"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/registry"
	helmrepo "helm.sh/helm/v3/pkg/repo"
	"strconv"
	"strings"
	"time"
)

type SyncService interface {
	Sync() (interface{}, error)
}

type SyncServiceImpl struct {
	chartRepoRepository                  sql.ChartRepoRepository
	logger                               *zap.SugaredLogger
	helmRepoManager                      HelmRepoManager
	dockerArtifactStoreRepository        sql.DockerArtifactStoreRepository
	ociRegistryConfigRepository          sql.OCIRegistryConfigRepository
	appStoreRepository                   sql.AppStoreRepository
	appStoreApplicationVersionRepository sql.AppStoreApplicationVersionRepository
	configuration                        *internal.Configuration
}

func NewSyncServiceImpl(chartRepoRepository sql.ChartRepoRepository,
	logger *zap.SugaredLogger,
	helmRepoManager HelmRepoManager,
	dockerArtifactStoreRepository sql.DockerArtifactStoreRepository,
	ociRegistryConfigRepository sql.OCIRegistryConfigRepository,
	appStoreRepository sql.AppStoreRepository,
	appStoreApplicationVersionRepository sql.AppStoreApplicationVersionRepository,
	configuration *internal.Configuration) *SyncServiceImpl {
	return &SyncServiceImpl{
		chartRepoRepository:                  chartRepoRepository,
		logger:                               logger,
		helmRepoManager:                      helmRepoManager,
		dockerArtifactStoreRepository:        dockerArtifactStoreRepository,
		ociRegistryConfigRepository:          ociRegistryConfigRepository,
		appStoreRepository:                   appStoreRepository,
		appStoreApplicationVersionRepository: appStoreApplicationVersionRepository,
		configuration:                        configuration,
	}
}

func (impl *SyncServiceImpl) Sync() (interface{}, error) {
	var (
		err           error
		repos         []*sql.ChartRepo
		repo          *sql.ChartRepo
		chartRepoId   int
		ociRegistries []*sql.DockerArtifactStore
		ociRegistry   *sql.DockerArtifactStore
	)
	if impl.configuration.ChartProviderId == "*" {
		ociRegistries, err = impl.dockerArtifactStoreRepository.FindAllChartProviders()
		if err != nil {
			impl.logger.Errorw("err in getting OCI Registries list", "err", err)
		}
		repos, err = impl.chartRepoRepository.GetAll()
		if err != nil {
			impl.logger.Errorw("err in getting repo list", "err", err)
		}
	} else {
		if impl.configuration.IsOCIRegistry {
			ociRegistry, err = impl.dockerArtifactStoreRepository.FindOne(impl.configuration.ChartProviderId)
			if err != nil {
				impl.logger.Errorw("err in getting OCI Registries list", "err", err)
				return nil, err
			}
			ociRegistries = []*sql.DockerArtifactStore{ociRegistry}
		} else {
			chartRepoId, err = strconv.Atoi(impl.configuration.ChartProviderId)
			if err != nil {
				impl.logger.Errorw("err in parsing ChartProviderId", "err", err)
				return nil, err
			}
			repo, err = impl.chartRepoRepository.FindById(chartRepoId)
			if err != nil {
				impl.logger.Errorw("err in getting repo list", "err", err)
				return nil, err
			}
			repos = []*sql.ChartRepo{repo}
		}
	}
	for _, registryObj := range ociRegistries {
		// validation to avoid nil pointer
		if !util.IsValidRegistryChartConfiguration(registryObj) {
			impl.logger.Errorw("no valid configuration found for OCI registry", "OCI registry", registryObj.Id)
			continue
		}
		impl.logger.Infow("syncing repo", "OCI Registry Id", registryObj.Id)
		err := impl.syncOCIRepo(registryObj)
		if err != nil {
			impl.logger.Errorw("repo sync error", "OCIRegistry", registryObj)
		}
	}
	for _, repository := range repos {
		impl.logger.Infow("syncing repo", "name", repository.Name)
		err := impl.syncChartRepo(repository)
		if err != nil {
			impl.logger.Errorw("repo sync error", "repo", repository)
		}
	}
	return nil, nil
}

func extractChartRepoRepositoryList(repositoryList string) []string {
	chartNameList := make([]string, 0)
	chartRepoRepositoryList := strings.Split(repositoryList, ",")
	for _, chartName := range chartRepoRepositoryList {
		chartNameList = append(chartNameList, strings.TrimSpace(chartName))
	}
	return chartNameList
}

func (impl *SyncServiceImpl) syncOCIRepo(ociRepo *sql.DockerArtifactStore) error {

	chartRepoRepositoryList := extractChartRepoRepositoryList(ociRepo.OCIRegistryConfig[0].RepositoryList)

	// marking all repos not present in request(chartRepoRepositoryList) for registry (ociRepo.Id) as inactive
	err := impl.appStoreRepository.MarkReposInactive(ociRepo.Id, chartRepoRepositoryList)
	if err != nil {
		impl.logger.Errorw("error in updating app store", "err", err)
		return nil
	}
	var appStoreRepos []*sql.AppStore
	var chartNames []string
	for _, chartName := range chartRepoRepositoryList {
		app := &sql.AppStore{
			Name:                  chartName,
			DockerArtifactStoreId: ociRepo.Id,
			CreatedOn:             time.Now(),
			UpdatedOn:             time.Now(),
			Active:                true,
		}
		appStoreRepos = append(appStoreRepos, app)
		chartNames = append(chartNames, chartName)
	}
	err = impl.appStoreRepository.Save(appStoreRepos)
	if err != nil {
		impl.logger.Errorw("error in inserting repos in app store", "err", err)
		return err
	}
	appStoreRepos, err = impl.appStoreRepository.GetAppStoresForOCIRepo(ociRepo.Id, chartNames)
	if err != nil {
		impl.logger.Errorw("error in fetching repos in app store", "err", err)
		return err
	}
	// get registry client for oci repo
	client, err := impl.getOciRegistryClient(ociRepo)
	if err != nil {
		impl.logger.Errorw("error in getting registry client for oci repo", "err", err)
		return err
	}
	for _, appStore := range appStoreRepos {
		ref := fmt.Sprintf("%s/%s", strings.TrimSpace(ociRepo.RegistryURL), appStore.Name)
		chartVersions, err := impl.helmRepoManager.FetchOCIChartTagsList(client, ref)
		if err != nil {
			impl.logger.Errorw("error in fetching OCI repository tags", "repository url", ref, "err", err)
			continue
		}
		impl.logger.Infow("handling all versions of chart", "registryName", ociRepo.Id, "chartName", appStore.Name, "chartVersions", chartVersions)
		err = impl.updateOCIRegistryChartVersions(client, appStore.Id, chartVersions, ociRepo, appStore.Name)
		if err != nil {
			impl.logger.Errorw("error in updating chart versions", "err", err, "appId", appStore.Id)
			continue
		}
	}
	return nil
}

func (impl *SyncServiceImpl) getOciRegistryClient(ociRepo *sql.DockerArtifactStore) (*registry.Client, error) {
	client, err := registry.NewClient()
	if err != nil {
		return client, err
	}
	username, password := "", ""
	if !ociRepo.OCIRegistryConfig[0].IsPublic {
		username, password, err = impl.helmRepoManager.ExtractCredentialsForRegistry(ociRepo)
		if err != nil {
			impl.logger.Errorw("error extracting AWS credentials", "registry id", ociRepo.Id, "err", err)
			return client, err
		}
		err = impl.helmRepoManager.RegistryLogin(client, ociRepo, username, password)
		if err != nil {
			impl.logger.Errorw("error logging in to OCI registry", "registry id", ociRepo.Id, "err", err)
			return client, err
		}
	}
	return client, nil
}

func (impl *SyncServiceImpl) syncChartRepo(repo *sql.ChartRepo) error {
	indexFile, err := impl.helmRepoManager.LoadIndexFile(repo)
	if err != nil {
		impl.logger.Errorw("error in loading index file", "repo", repo.Name, "err", err)
		return err
	}
	indexFile.SortEntries()

	var appStores []*sql.AppStore
	IndexFileEntries := indexFile.Entries

	var chartNames []string
	for name, _ := range IndexFileEntries {
		//new app create AppStore
		app := &sql.AppStore{
			Name:        name,
			ChartRepoId: repo.Id,
			CreatedOn:   time.Now(),
			UpdatedOn:   time.Now(),
			Active:      true,
		}
		appStores = append(appStores, app)
		chartNames = append(chartNames, name)
		//update entries if any  id, chartVersions
	}
	err = impl.appStoreRepository.Save(appStores)
	if err != nil {
		impl.logger.Errorw("error in saving apps", "err", err)
		return err
	}

	appStores, err = impl.appStoreRepository.GetAppStoresForChartRepo(repo.Id, chartNames)
	if err != nil {
		impl.logger.Errorw("error in fetching repos in app store", "err", err)
		return err
	}

	appStoreMap := make(map[string]int)
	for _, app := range appStores {
		appStoreMap[app.Name] = app.Id
	}

	for name, chartVersions := range IndexFileEntries {
		impl.logger.Infow("handling all versions of chart", "repoName", repo.Name, "chartName", name, "chartVersions", len(chartVersions))
		err := impl.updateChartVersions(appStoreMap[name], &chartVersions, repo.Url, repo.Username, repo.Password, repo.AllowInsecureConnection)
		if err != nil {
			impl.logger.Errorw("error in updating chart versions", "err", err, "appId", appStoreMap[name])
			continue
		}
	}

	return nil
}

func (impl *SyncServiceImpl) updateChartVersions(appId int, chartVersions *helmrepo.ChartVersions, repoUrl string, username string, password string, allowInsecureConnection bool) error {
	applicationVersions, err := impl.appStoreApplicationVersionRepository.FindVersionsByAppStoreId(appId)
	if err != nil {
		impl.logger.Errorw("error in getting application versions ", "err", err, "appId", appId)
		return err
	}
	applicationVersionMaps := make(map[string]int)

	for _, applicationVersion := range applicationVersions {
		applicationVersionMaps[applicationVersion.Version] = applicationVersion.Id
	}
	var appVersions []*sql.AppStoreApplicationVersion
	var isAnyChartVersionFound bool
	for _, chartVersion := range *chartVersions {
		if _, ok := applicationVersionMaps[chartVersion.Version]; ok {
			//already present
			impl.logger.Warnw("ignoring chart version as this already exists", "appStoreId", appId, "chartVersion", chartVersion.Version)
			break
		}
		chartVersionJson, err := json.Marshal(chartVersion)
		if err != nil {
			impl.logger.Errorw("error in marshaling json", "err", err)
			continue
		}
		rawValues, readme, valuesSchemaJson, notes, err := impl.helmRepoManager.ValuesJson(repoUrl, chartVersion, username, password, allowInsecureConnection)
		if err != nil {
			impl.logger.Errorw("error in getting values yaml", "err", err)
			continue
		}

		jsonByte, err := yaml.YAMLToJSON([]byte(rawValues))
		if err != nil {
			impl.logger.Errorw("error in getting values yaml", "err", err)
			continue
		}

		if !isAnyChartVersionFound {
			isAnyChartVersionFound = true
		}

		application := &sql.AppStoreApplicationVersion{
			Id:          0,
			Version:     chartVersion.Version,
			AppVersion:  chartVersion.AppVersion,
			Created:     chartVersion.Created,
			Deprecated:  chartVersion.Deprecated,
			Description: chartVersion.Description,
			Digest:      chartVersion.Digest,
			Icon:        chartVersion.Icon,
			Name:        chartVersion.Name,
			//Source:      chartVersion.Sources, //FIXME
			Home:       chartVersion.Home,
			ValuesYaml: string(jsonByte),
			ChartYaml:  string(chartVersionJson),
			Latest:     false,
			AppStoreId: appId,
			AuditLog: sql.AuditLog{
				CreatedOn: time.Now(),
				UpdatedOn: time.Now(),
				CreatedBy: 1,
				UpdatedBy: 1,
			},
			RawValues:        rawValues,
			Readme:           readme,
			ValuesSchemaJson: valuesSchemaJson,
			Notes:            notes,
			AppStore:         nil,
		}
		appVersions = append(appVersions, application)

		// save 20 versions and reset the array (as memory would go increasing if save on one-go)
		if len(appVersions) == impl.configuration.AppStoreAppVersionsSaveChunkSize {
			// save into DB
			impl.logger.Infow("saving chart versions into DB", "versions", len(appVersions))
			isNewChartVersionFound, err := impl.appStoreApplicationVersionRepository.Save(&appVersions)
			if err != nil {
				impl.logger.Errorw("error in updating", "totalIn", len(*chartVersions), "totalOut", len(appVersions), "err", err)
				return err
			}
			if !isAnyChartVersionFound {
				isAnyChartVersionFound = isNewChartVersionFound
			}
			// reset the array
			appVersions = nil
		}
	}

	if !isAnyChartVersionFound {
		impl.logger.Infow("no change for ", "app", appId)
		return nil
	}

	// if any version left to save
	if len(appVersions) > 0 {
		impl.logger.Infow("saving remaining chart versions into DB", "versions", len(appVersions))
		_, err = impl.appStoreApplicationVersionRepository.Save(&appVersions)
		if err != nil {
			impl.logger.Errorw("error in updating", "totalIn", len(*chartVersions), "totalOut", len(appVersions), "err", err)
			return err
		}
	}

	var latestFlagAppVersions []*sql.AppStoreApplicationVersion
	latestCreated, err := impl.appStoreApplicationVersionRepository.FindLatestCreated(appId)
	if err != nil {
		impl.logger.Errorw("error in marking latest", "err", err)
		return err
	}
	latestCreated.Latest = true
	latestFlagAppVersions = append(latestFlagAppVersions, latestCreated)
	application, err := impl.appStoreApplicationVersionRepository.FindLatest(appId)
	if err != nil && err != pg.ErrNoRows {
		impl.logger.Errorw("error in marking latest", "err", err)
		return err
	}
	if err == nil {
		application.Latest = false
		latestFlagAppVersions = append(latestFlagAppVersions, application)
	}
	err = impl.appStoreApplicationVersionRepository.Update(latestFlagAppVersions)
	if err != nil {
		impl.logger.Errorw("error in marking latest", "err", err)
		return err
	}
	return nil
}

func (impl *SyncServiceImpl) updateOCIRegistryChartVersions(client *registry.Client, appId int, chartVersions []string, ociRepo *sql.DockerArtifactStore, chartName string) error {
	chartVersionsCount := len(chartVersions)
	applicationVersions, err := impl.appStoreApplicationVersionRepository.FindVersionsByAppStoreId(appId)
	if err != nil {
		impl.logger.Errorw("error in getting application versions ", "err", err, "appId", appId)
		return err
	}
	applicationVersionMaps := make(map[string]int)

	for _, applicationVersion := range applicationVersions {
		applicationVersionMaps[applicationVersion.Version] = applicationVersion.Id
	}
	var appVersions []*sql.AppStoreApplicationVersion
	var isAnyChartVersionFound bool
	for _, chartVersion := range chartVersions {
		if _, ok := applicationVersionMaps[chartVersion]; ok {
			//already present
			impl.logger.Warnw("ignoring chart version as this already exists", "appStoreId", appId, "chartVersion", chartVersion)
			continue
		}
		if !isAnyChartVersionFound {
			isAnyChartVersionFound = true
		}
		chartVersionJson, err := json.Marshal(chartVersion)
		if err != nil {
			impl.logger.Errorw("error in marshaling json", "err", err)
			continue
		}
		metaData, rawValues, readme, valuesSchemaJson, notes, diagest, err := impl.helmRepoManager.OCIRepoValuesJson(client, ociRepo.RegistryURL, chartName, chartVersion)
		if err != nil {
			impl.logger.Errorw("error in getting values yaml", "err", err)
			continue
		}
		jsonByte, err := yaml.YAMLToJSON([]byte(rawValues))
		if err != nil {
			impl.logger.Errorw("error in getting values yaml", "err", err)
			continue
		}

		application := &sql.AppStoreApplicationVersion{
			Id:          0,
			Version:     chartVersion,
			Description: metaData.Description,
			AppVersion:  metaData.AppVersion,
			Digest:      diagest,
			Icon:        metaData.Icon,
			Home:        metaData.Home,
			Deprecated:  metaData.Deprecated,
			Name:        metaData.Name,
			ValuesYaml:  string(jsonByte),
			ChartYaml:   string(chartVersionJson),
			Latest:      false,
			AppStoreId:  appId,
			AuditLog: sql.AuditLog{
				CreatedOn: time.Now(),
				UpdatedOn: time.Now(),
				CreatedBy: 1,
				UpdatedBy: 1,
			},
			RawValues:        rawValues,
			Readme:           readme,
			ValuesSchemaJson: valuesSchemaJson,
			Notes:            notes,
			AppStore:         nil,
		}
		appVersions = append(appVersions, application)

		// save 20 versions and reset the array (as memory would go increasing if save on one-go)
		if len(appVersions) == impl.configuration.AppStoreAppVersionsSaveChunkSize {
			// save into DB
			impl.logger.Infow("saving chart versions into DB", "versions", len(appVersions))
			_, err := impl.appStoreApplicationVersionRepository.Save(&appVersions)
			if err != nil {
				impl.logger.Errorw("error in updating", "totalIn", chartVersionsCount, "totalOut", len(appVersions), "err", err)
				return err
			}
			// reset the array
			appVersions = nil
		}
	}

	if !isAnyChartVersionFound {
		impl.logger.Infow("no change for ", "app", appId)
		return nil
	}

	// if any version left to save
	if len(appVersions) > 0 {
		impl.logger.Infow("saving remaining chart versions into DB", "versions", len(appVersions))
		_, err := impl.appStoreApplicationVersionRepository.Save(&appVersions) // db unique constraint, upsert
		if err != nil {
			impl.logger.Errorw("error in updating", "totalIn", chartVersionsCount, "totalOut", len(appVersions), "err", err)
			return err
		}
	}
	// Update latest version for the chart
	if chartVersionsCount > 0 {
		var latestFlagAppVersions []*sql.AppStoreApplicationVersion
		latestChartVersion := chartVersions[0]
		latestCreated, err := impl.appStoreApplicationVersionRepository.FindOneByAppStoreIdAndVersion(appId, latestChartVersion)
		if err != nil {
			impl.logger.Errorw("error in marking latest", "err", err)
			return err
		}
		latestCreated.Latest = true
		latestFlagAppVersions = append(latestFlagAppVersions, latestCreated)
		application, err := impl.appStoreApplicationVersionRepository.FindLatest(appId)
		if err != nil && err != pg.ErrNoRows {
			impl.logger.Errorw("error in marking latest", "err", err)
			return err
		}
		if application.Id == latestCreated.Id {
			return nil
		}
		if err == nil {
			application.Latest = false
			latestFlagAppVersions = append(latestFlagAppVersions, application)
		}
		err = impl.appStoreApplicationVersionRepository.Update(latestFlagAppVersions)
		if err != nil {
			impl.logger.Errorw("error in marking latest", "err", err)
			return err
		}
	}
	return nil
}
