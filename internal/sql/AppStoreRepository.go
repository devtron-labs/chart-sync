package sql

import (
	"github.com/go-pg/pg"
	"go.uber.org/zap"
	"time"
)

type AppStoreRepository interface {
	FindByRepoId(repoId int) (appStores []*AppStore, err error)
	Save(appStore *AppStore) error
}

type AppStoreRepositoryImpl struct {
	dbConnection *pg.DB
	Logger       *zap.SugaredLogger
}

func NewAppStoreRepositoryImpl(Logger *zap.SugaredLogger, dbConnection *pg.DB) *AppStoreRepositoryImpl {
	return &AppStoreRepositoryImpl{dbConnection: dbConnection, Logger: Logger}
}

type AppStore struct {
	TableName        struct{}  `sql:"app_store"`
	Id               int       `sql:"id,pk"`
	Name             string    `sql:"name"`
	ChartRepoId      int       `sql:"chart_repo_id"`
	Active           bool      `sql:"active"`
	ChartGitLocation string    `sql:"chart_git_location"`
	CreatedOn        time.Time `sql:"created_on"`
	UpdatedOn        time.Time `sql:"updated_on"`
	ChartRepo        ChartRepo
}

func (impl *AppStoreRepositoryImpl) FindByRepoId(repoId int) (appStores []*AppStore, err error) {
	err = impl.dbConnection.Model(&appStores).Where("chart_repo_id =?", repoId).Select()
	return appStores, err
}

func (impl *AppStoreRepositoryImpl) Save(appStore *AppStore) error {
	return impl.dbConnection.Insert(appStore)

}
