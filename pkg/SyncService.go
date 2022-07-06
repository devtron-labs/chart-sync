package pkg

import (
	"encoding/json"
	"github.com/devtron-labs/chart-sync/internal/sql"
	"github.com/ghodss/yaml"
	"github.com/go-pg/pg"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/repo"
	"time"
)

type SyncService interface {
	Sync() (interface{}, error)
}
type SyncServiceImpl struct {
	chartRepoRepository                  sql.ChartRepoRepository
	logger                               *zap.SugaredLogger
	helmRepoManager                      HelmRepoManager
	appStoreRepository                   sql.AppStoreRepository
	appStoreApplicationVersionRepository sql.AppStoreApplicationVersionRepository
}

func NewSyncServiceImpl(chartRepoRepository sql.ChartRepoRepository,
	logger *zap.SugaredLogger,
	helmRepoManager HelmRepoManager,
	appStoreRepository sql.AppStoreRepository,
	appStoreApplicationVersionRepository sql.AppStoreApplicationVersionRepository) *SyncServiceImpl {
	return &SyncServiceImpl{
		chartRepoRepository:                  chartRepoRepository,
		logger:                               logger,
		helmRepoManager:                      helmRepoManager,
		appStoreRepository:                   appStoreRepository,
		appStoreApplicationVersionRepository: appStoreApplicationVersionRepository,
	}
}
func (impl *SyncServiceImpl) Sync() (interface{}, error) {
	repos, err := impl.chartRepoRepository.GetAll()
	if err != nil {
		impl.logger.Errorw("err in getting repo list", "err", err)
		return nil, err
	}
	for _, repo := range repos {
		impl.logger.Infow("snehith testing sync", "repo id", repo.Id, "repo name", repo.Name)
		err := impl.syncRepo(repo)
		if err != nil {
			impl.logger.Errorw("repo sync error", "repo", repo)
		}
	}
	return nil, nil
}

func (impl *SyncServiceImpl) syncRepo(repo *sql.ChartRepo) error {
	impl.logger.Infow("snehith testing syncrepo ", "repo", repo)
	indexFile, err := impl.helmRepoManager.LoadIndexFile(repo)
	if err != nil {
		impl.logger.Errorw("error in loading index file", "repo", repo.Name, "err", err)
		return err
	}
	impl.logger.Infow("snehith testing syncrepo1 ", "indexFile", indexFile)
	indexFile.SortEntries()
	applications, err := impl.appStoreRepository.FindByRepoId(repo.Id)
	impl.logger.Infow("snehith testing syncrepo2 ", "applications", applications)
	if err != nil {
		impl.logger.Errorw("error in fetching app for repo", "repo", repo.Id, "err", err)
	}
	applicationId := make(map[string]int)
	for _, application := range applications {
		applicationId[application.Name] = application.Id
	}

	for name, chartVersions := range indexFile.Entries {
		impl.logger.Infow("snehith testing", "name", name, "entries length", len(indexFile.Entries), "repo Id ", repo.Id)
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
			impl.logger.Infow("testing saving app", "app", app)
			err = impl.appStoreRepository.Save(app)
			if err != nil {
				impl.logger.Errorw("error in saving app", "app", app, "err", err)
				continue
			}
			applicationId[name] = app.Id
			id = app.Id
		}
		//update entries if any  id, chartVersions
		impl.logger.Infow("updating app", "name", name)
		err := impl.updateChartVersions(id, &chartVersions, repo.Url)
		if err != nil {
			impl.logger.Errorw("error in updating chart versions", "err", err, "appId", id)
			continue
		}
	}
	return nil
}

func (impl *SyncServiceImpl) updateChartVersions(appId int, chartVersions *repo.ChartVersions, baseurl string) error {
	impl.logger.Infow("snehit updateChartVersions", "appId", appId)
	applicationVersions, err := impl.appStoreApplicationVersionRepository.FindVersionsByAppStoreId(appId)
	if err != nil {
		impl.logger.Errorw("error in getting application versions ", "err", err, "appId", appId)
		return err
	}
	applicationVersionMaps := make(map[string]int)
	impl.logger.Infow("snehit updateChartVersions1")
	for _, applicationVersion := range applicationVersions {
		applicationVersionMaps[applicationVersion.Version] = applicationVersion.Id
	}
	var appVersions []*sql.AppStoreApplicationVersion
	for _, chartVersion := range *chartVersions {
		//testing
		//if _, ok := applicationVersionMaps[chartVersion.Version]; ok {
		//	//already present
		//	break
		//}
		chartVersionJson, err := json.Marshal(chartVersion)
		if err != nil {
			impl.logger.Errorw("error in marshaling json", "err", err)
			//continue
		}
		rawValues, readme, schemaJson, notes, err := impl.helmRepoManager.ValuesJson(baseurl, chartVersion)
		if err != nil {
			impl.logger.Errorw("error in getting values yaml", "err", err)
			impl.logger.Errorw(schemaJson) //testing
			//continue
		}

		jsonByte, err := yaml.YAMLToJSON([]byte(rawValues))
		if err != nil {
			impl.logger.Errorw("error in getting values yaml", "err", err)
			//continue
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
			RawValues:  rawValues,
			Readme:     readme,
			SchemaJson: schemaJson,
			Notes:      notes,
			AppStore:   nil,
		}
		appVersions = append(appVersions, application)
	}
	if len(appVersions) == 0 {
		impl.logger.Infow("no change for ", "app", appId)
		return nil
	}
	err = impl.appStoreApplicationVersionRepository.Save(&appVersions)
	if err != nil {
		impl.logger.Errorw("error in updating", "totalIn", len(*chartVersions), "totalOut", len(appVersions), "err", err)
		return err
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
