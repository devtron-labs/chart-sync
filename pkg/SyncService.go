package pkg

import (
	"encoding/json"
	"fmt"
	"github.com/devtron-labs/chart-sync/internals"
	"github.com/devtron-labs/chart-sync/internals/sql"
	registry2 "github.com/devtron-labs/chart-sync/pkg/registry"
	"github.com/devtron-labs/chart-sync/util"
	registry3 "github.com/devtron-labs/common-lib/helmLib/registry"
	"github.com/ghodss/yaml"
	"github.com/go-pg/pg"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/repo"
	url2 "net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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
	configuration                        *internals.Configuration
	registrySettings                     registry3.SettingsFactory
	mutex                                sync.Mutex
}

func NewSyncServiceImpl(chartRepoRepository sql.ChartRepoRepository,
	logger *zap.SugaredLogger,
	helmRepoManager HelmRepoManager,
	dockerArtifactStoreRepository sql.DockerArtifactStoreRepository,
	ociRegistryConfigRepository sql.OCIRegistryConfigRepository,
	appStoreRepository sql.AppStoreRepository,
	appStoreApplicationVersionRepository sql.AppStoreApplicationVersionRepository,
	configuration *internals.Configuration,
	registrySettings registry3.SettingsFactory,
) *SyncServiceImpl {
	return &SyncServiceImpl{
		chartRepoRepository:                  chartRepoRepository,
		logger:                               logger,
		helmRepoManager:                      helmRepoManager,
		dockerArtifactStoreRepository:        dockerArtifactStoreRepository,
		ociRegistryConfigRepository:          ociRegistryConfigRepository,
		appStoreRepository:                   appStoreRepository,
		appStoreApplicationVersionRepository: appStoreApplicationVersionRepository,
		configuration:                        configuration,
		registrySettings:                     registrySettings,
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
		err := impl.syncRepo(repository)
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
	applications, err := impl.appStoreRepository.FindByStoreId(ociRepo.Id)
	if err != nil {
		impl.logger.Errorw("error in fetching app for repo", "OCI registry", ociRepo.Id, "err", err)
		return nil
	}
	applicationId := make(map[string]int)
	// Already validated for nil pointer
	chartRepoRepositoryList := extractChartRepoRepositoryList(ociRepo.OCIRegistryConfig[0].RepositoryList)
	removedApplicationList := make([]*sql.AppStore, 0)
	for _, application := range applications {
		if !slices.Contains(chartRepoRepositoryList, application.Name) {
			application.Active = false
			application.UpdatedOn = time.Now()
			removedApplicationList = append(removedApplicationList, application)
		}
		applicationId[application.Name] = application.Id
	}

	if len(removedApplicationList) > 0 {
		impl.logger.Errorw("removing in charts from app store", "RemovedApplicationList", removedApplicationList, "err", err)
		err = impl.appStoreRepository.Update(removedApplicationList)
		if err != nil {
			impl.logger.Errorw("error in updating app store", "err", err)
			return nil
		}
	}
	registryConfig, err := registry2.NewToRegistryConfig(ociRepo)
	defer func() {
		if registryConfig != nil {
			err := registry3.DeleteCertificateFolder(registryConfig.RegistryCAFilePath)
			if err != nil {
				impl.logger.Errorw("error in deleting certificate folder", "registryName", registryConfig.RegistryId, "err", err)
			}
		}
	}()
	if err != nil {
		impl.logger.Errorw("error in getting registry config", "registryName", registryConfig.RegistryId, "err", err)
		return nil
	}
	settingsGetter, err := impl.registrySettings.GetSettings(registryConfig)
	if err != nil {
		impl.logger.Errorw("error in getting registry settings", "registryName", registryConfig.RegistryId, "err", err)
		return nil
	}
	settings, err := settingsGetter.GetRegistrySettings(registryConfig)
	if err != nil {
		impl.logger.Errorw("error in getting registry settings for registry", "registryName", ociRepo.Id, "err", err)
		return err
	}
	client := settings.RegistryClient
	ociRepo.RegistryURL = settings.RegistryHostURL

	for _, chartName := range chartRepoRepositoryList {
		var url *url2.URL
		if !strings.Contains(strings.ToLower(ociRepo.RegistryURL), "https") && !strings.Contains(strings.ToLower(ociRepo.RegistryURL), "http") {
			url, err = url2.Parse(fmt.Sprintf("//%s", ociRepo.RegistryURL))
		} else {
			url, err = url2.Parse(ociRepo.RegistryURL)
		}

		if err != nil {
			impl.logger.Errorw("registry url parse err", "registryURL", ociRepo.RegistryURL, "err", err)
			return err
		}
		parsedUrlPath := strings.TrimSpace(url.Path)
		parsedHost := strings.TrimSpace(url.Host)
		parsedRepoName := strings.TrimSpace(chartName)
		// Join handles empty strings
		ref := filepath.Join(parsedHost, parsedUrlPath, parsedRepoName)
		var chartVersions []string

		chartVersions, err = impl.helmRepoManager.FetchOCIChartTagsList(settings, ref)
		if err != nil {
			impl.logger.Errorw("error in fetching OCI repository tags", "repository url", ref, "err", err)
			continue
		}

		id, ok := applicationId[chartName]
		if !ok {
			app, fetchErr := impl.appStoreRepository.FindInactiveOneByName(ociRepo.Id, chartName)
			if fetchErr == nil {
				app.Active = true
				app.UpdatedOn = time.Now()
				err = impl.appStoreRepository.Update([]*sql.AppStore{app})
				if err != nil {
					impl.logger.Errorw("error in updating app store", "err", err)
					continue
				}
			} else if fetchErr == pg.ErrNoRows {
				//create new app in AppStore
				app = &sql.AppStore{
					Name:                  chartName,
					DockerArtifactStoreId: ociRepo.Id,
					CreatedOn:             time.Now(),
					UpdatedOn:             time.Now(),
					Active:                true,
				}
				err = impl.appStoreRepository.Save(app)
				if err != nil {
					impl.logger.Errorw("error in saving app", "app", app, "err", err)
					continue
				}
			} else {
				continue
			}
			applicationId[chartName] = app.Id
			id = app.Id
		}
		//update entries if any  id, chartVersions
		impl.logger.Infow("handling all versions of chart", "registryName", ociRepo.Id, "chartName", chartName, "chartVersions", len(chartVersions))
		if impl.configuration.ParallelismLimitForTagProcessing == 0 {
			err = impl.updateOCIRegistryChartVersions(client, id, chartVersions, ociRepo, chartName)
		} else {
			err = impl.updateOCIRegistryChartVersionsV2(client, id, chartVersions, ociRepo, chartName)
		}
		if err != nil {
			impl.logger.Errorw("error in updating chart versions", "err", err, "appId", id)
			continue
		}
	}
	return nil
}

func (impl *SyncServiceImpl) syncRepo(repo *sql.ChartRepo) error {
	indexFile, err := impl.helmRepoManager.LoadIndexFile(repo)
	if err != nil {
		impl.logger.Errorw("error in loading index file", "repo", repo.Name, "err", err)
		return err
	}
	indexFile.SortEntries()
	applications, err := impl.appStoreRepository.FindByRepoId(repo.Id)
	if err != nil {
		impl.logger.Errorw("error in fetching app for repo", "repo", repo.Id, "err", err)
	}
	applicationId := make(map[string]int)
	for _, application := range applications {
		applicationId[application.Name] = application.Id
	}
	for name, chartVersions := range indexFile.Entries {
		id, ok := applicationId[name]
		if !ok {
			//new app create AppStore
			app := &sql.AppStore{
				Name:        name,
				ChartRepoId: repo.Id,
				CreatedOn:   time.Now(),
				UpdatedOn:   time.Now(),
				Active:      true,
			}
			err = impl.appStoreRepository.Save(app)
			if err != nil {
				impl.logger.Errorw("error in saving app", "app", app, "err", err)
				continue
			}
			applicationId[name] = app.Id
			id = app.Id
		}
		//update entries if any  id, chartVersions
		impl.logger.Infow("handling all versions of chart", "repoName", repo.Name, "chartName", name, "chartVersions", len(chartVersions))
		err := impl.updateChartVersions(id, &chartVersions, repo.Url, repo.Username, repo.Password, repo.AllowInsecureConnection)
		if err != nil {
			impl.logger.Errorw("error in updating chart versions", "err", err, "appId", id)
			continue
		}
	}
	return nil
}

func (impl *SyncServiceImpl) updateChartVersions(appId int, chartVersions *repo.ChartVersions, repoUrl string, username string, password string, allowInsecureConnection bool) error {
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

		if chartVersion.Created.IsZero() {
			// Created field is used in marking chart latest, so updating it with current time if it null
			chartVersion.Created = time.Now()
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
			err = impl.appStoreApplicationVersionRepository.Save(&appVersions)
			if err != nil {
				impl.logger.Errorw("error in updating", "totalIn", len(*chartVersions), "totalOut", len(appVersions), "err", err)
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
		err = impl.appStoreApplicationVersionRepository.Save(&appVersions)
		if err != nil {
			impl.logger.Errorw("error in updating", "totalIn", len(*chartVersions), "totalOut", len(appVersions), "err", err)
			return err
		}
	}

	return nil
}

func (impl *SyncServiceImpl) updateOCIRegistryChartVersions(client *registry.Client, appId int, chartVersions []string, ociRepo *sql.DockerArtifactStore, chartName string) error {

	chartVersionsCount := len(chartVersions)

	newChartVersions, err := impl.getNewChartVersions(appId, chartVersions)
	if err != nil {
		impl.logger.Errorw("error in getting new chart versions", "appStoreId", appId, "err", err)
		return err
	}

	var appVersions []*sql.AppStoreApplicationVersion
	var isAnyChartVersionFound bool
	for _, chartVersion := range newChartVersions {

		chartData, err := impl.helmRepoManager.OCIRepoValuesJson(client, ociRepo.RegistryURL, chartName, chartVersion)
		if err != nil {
			impl.logger.Errorw("error in getting values yaml", "err", err)
			continue
		}

		if !isAnyChartVersionFound {
			isAnyChartVersionFound = true
		}

		application, err := impl.parseAppStoreApplicationDbObj(chartVersion, chartData, appId)
		if err != nil {
			impl.logger.Errorw("error in parsing app store application object", "appStoreId", appId, "chartVersion", chartVersion, "err", err)
			return err
		}
		appVersions = append(appVersions, application)

		// save 20 versions and reset the array (as memory would go increasing if save on one-go)
		if len(appVersions) == impl.configuration.AppStoreAppVersionsSaveChunkSize {
			// save into DB
			impl.logger.Infow("saving chart versions into DB", "versions", len(appVersions))
			err = impl.appStoreApplicationVersionRepository.Save(&appVersions)
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
		err = impl.appStoreApplicationVersionRepository.Save(&appVersions)
		if err != nil {
			impl.logger.Errorw("error in updating", "totalIn", chartVersionsCount, "totalOut", len(appVersions), "err", err)
			return err
		}
	}
	return nil
}

func (impl *SyncServiceImpl) getNewChartVersions(appId int, chartVersions []string) ([]string, error) {
	applicationVersionMaps, err := impl.getApplicationVersionsMapping(appId)
	if err != nil {
		impl.logger.Errorw("error in getting application versions mapping")
		return nil, err
	}
	newChartVersions := make([]string, 0)
	for _, chartVersion := range chartVersions {
		if _, ok := applicationVersionMaps[chartVersion]; ok {
			//already present
			impl.logger.Warnw("ignoring chart version as this already exists", "appStoreId", appId, "chartVersion", chartVersion)
			continue
		}
		newChartVersions = append(newChartVersions, chartVersion)
	}
	return newChartVersions, nil
}

func (impl *SyncServiceImpl) getApplicationVersionsMapping(appStoreId int) (map[string]int, error) {
	applicationVersions, err := impl.appStoreApplicationVersionRepository.FindVersionsByAppStoreId(appStoreId)
	if err != nil {
		impl.logger.Errorw("error in getting application versions ", "err", err, "appId", appStoreId)
		return nil, err
	}
	applicationVersionMaps := make(map[string]int)
	for _, applicationVersion := range applicationVersions {
		applicationVersionMaps[applicationVersion.Version] = applicationVersion.Id
	}
	return applicationVersionMaps, nil
}

func (impl *SyncServiceImpl) parseAppStoreApplicationDbObj(chartVersion string, chartData ChartData, appId int) (*sql.AppStoreApplicationVersion, error) {

	jsonByte, err := yaml.YAMLToJSON([]byte(chartData.RawValues))
	if err != nil {
		impl.logger.Errorw("error in getting values yaml", "err", err)
		return nil, err
	}

	chartVersionJson, err := json.Marshal(chartVersion)
	if err != nil {
		impl.logger.Errorw("error in marshaling json", "err", err)
		return nil, err
	}

	application := &sql.AppStoreApplicationVersion{
		Id:          0,
		Version:     chartVersion,
		Description: chartData.MetaData.Description,
		AppVersion:  chartData.MetaData.AppVersion,
		Digest:      chartData.Digest,
		Icon:        chartData.MetaData.Icon,
		Home:        chartData.MetaData.Home,
		Deprecated:  chartData.MetaData.Deprecated,
		Name:        chartData.MetaData.Name,
		ValuesYaml:  string(jsonByte),
		ChartYaml:   string(chartVersionJson),
		AppStoreId:  appId,
		AuditLog: sql.AuditLog{
			CreatedOn: time.Now(),
			UpdatedOn: time.Now(),
			CreatedBy: 1,
			UpdatedBy: 1,
		},
		RawValues:        chartData.RawValues,
		Readme:           chartData.Readme,
		ValuesSchemaJson: chartData.ValuesSchemaJson,
		Notes:            chartData.Notes,
		AppStore:         nil,
	}
	return application, nil
}

func (impl *SyncServiceImpl) updateOCIRegistryChartVersionsV2(client *registry.Client, appId int, chartVersions []string, ociRepo *sql.DockerArtifactStore, chartName string) error {

	newChartVersions, err := impl.getNewChartVersions(appId, chartVersions)
	if err != nil {
		impl.logger.Errorw("error in getting new chart versions", "appStoreId", appId, "err", err)
		return err
	}

	var isAnyChartVersionFound bool

	wg := new(sync.WaitGroup)

	workerPool := make(chan struct{}, impl.configuration.ParallelismLimitForTagProcessing)

	var appVersions []*sql.AppStoreApplicationVersion

	for _, cv := range newChartVersions {

		wg.Add(1)
		workerPool <- struct{}{}

		go func(client *registry.Client, registryURL, chartName string, chartVersion string) {

			defer func() {
				wg.Done()
				<-workerPool
			}()

			chartData, err := impl.helmRepoManager.OCIRepoValuesJson(client, ociRepo.RegistryURL, chartName, chartVersion)
			if err != nil {
				impl.logger.Errorw("error in getting values yaml", "err", err)
				return
			}

			if !isAnyChartVersionFound {
				isAnyChartVersionFound = true
			}

			application, err := impl.parseAppStoreApplicationDbObj(chartVersion, chartData, appId)
			if err != nil {
				impl.logger.Errorw("error in parsing app store application object", "appStoreId", appId, "chartVersion", chartVersion, "err", err)
				return
			}

			impl.mutex.Lock()
			defer impl.mutex.Unlock()
			appVersions = append(appVersions, application)
			if len(appVersions) == impl.configuration.AppStoreAppVersionsSaveChunkSize {
				impl.logger.Infow("saving chart versions into DB", "versions", len(appVersions))
				err = impl.appStoreApplicationVersionRepository.Save(&appVersions)
				if err != nil {
					impl.logger.Errorw("error in updating", len(appVersions), "err", err)
					return
				}
				appVersions = nil
			}

		}(client, ociRepo.RegistryURL, chartName, cv)

	}

	wg.Wait()

	if len(appVersions) > 0 {
		impl.logger.Infow("saving remaining chart versions into DB", "versions", len(appVersions))
		err = impl.appStoreApplicationVersionRepository.Save(&appVersions)
		if err != nil {
			impl.logger.Errorw("error in updating", len(appVersions), "err", err)
			return err
		}
	}

	if !isAnyChartVersionFound {
		impl.logger.Infow("no change for ", "app", appId)
		return nil
	}

	return nil
}
