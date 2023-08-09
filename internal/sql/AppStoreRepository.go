package sql

import (
	"github.com/go-pg/pg"
	"go.uber.org/zap"
	"time"
)

type AppStoreRepository interface {
	FindByStoreId(storeId string) (appStores []*AppStore, err error)
	FindInactiveOneByName(storeId, name string) (appStore *AppStore, err error)
	FindByRepoId(repoId int) (appStores []*AppStore, err error)
	Save(appStore *AppStore) error
	Update(appStore []*AppStore) error
}

type AppStoreRepositoryImpl struct {
	dbConnection *pg.DB
	Logger       *zap.SugaredLogger
}

func NewAppStoreRepositoryImpl(Logger *zap.SugaredLogger, dbConnection *pg.DB) *AppStoreRepositoryImpl {
	return &AppStoreRepositoryImpl{dbConnection: dbConnection, Logger: Logger}
}

type AppStore struct {
	TableName             struct{}  `sql:"app_store" pg:",discard_unknown_columns"`
	Id                    int       `sql:"id,pk"`
	Name                  string    `sql:"name,notnull"`
	ChartRepoId           int       `sql:"chart_repo_id"`
	DockerArtifactStoreId string    `sql:"docker_artifact_store_id"`
	Active                bool      `sql:"active,notnull"`
	ChartGitLocation      string    `sql:"chart_git_location"`
	CreatedOn             time.Time `sql:"created_on,notnull"`
	UpdatedOn             time.Time `sql:"updated_on,notnull"`
	ChartRepo             ChartRepo
}

func (impl *AppStoreRepositoryImpl) FindByRepoId(repoId int) (appStores []*AppStore, err error) {
	err = impl.dbConnection.Model(&appStores).
		Where("chart_repo_id =?", repoId).
		Where("active =?", true).
		Select()
	return appStores, err
}

func (impl *AppStoreRepositoryImpl) FindByStoreId(storeId string) (appStores []*AppStore, err error) {
	err = impl.dbConnection.Model(&appStores).
		Where("docker_artifact_store_id =?", storeId).
		Where("active =?", true).
		Select()
	return appStores, err
}
func (impl *AppStoreRepositoryImpl) FindInactiveOneByName(storeId, name string) (*AppStore, error) {
	appStore := AppStore{}
	err := impl.dbConnection.Model(&appStore).
		Where("docker_artifact_store_id =?", storeId).
		Where("name =?", name).
		Where("active =?", false).
		Select()
	if err != nil && err != pg.ErrNoRows {
		impl.Logger.Errorw("error in fetching inactive app for name", "ChartName", name, "err", err)
	}
	return &appStore, err
}

func (impl *AppStoreRepositoryImpl) Save(appStore *AppStore) error {
	return impl.dbConnection.Insert(appStore)
}

func (impl *AppStoreRepositoryImpl) Update(appStores []*AppStore) error {
	err := impl.dbConnection.RunInTransaction(func(tx *pg.Tx) error {
		for _, appStore := range appStores {
			_, err := tx.Model(appStore).WherePK().UpdateNotNull()
			if err != nil {
				return err
			}
		}
		return nil
	})
	return err
}
