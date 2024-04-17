package sql

import (
	"github.com/go-pg/pg"
	"go.uber.org/zap"
	"time"
)

type AppStoreApplicationVersionRepository interface {
	FindVersionsByAppStoreId(appStoreId int) ([]*AppStoreApplicationVersion, error)
	Save(versions *[]*AppStoreApplicationVersion) error
	FindOneByAppStoreIdAndVersion(appStoreId int, version string) (*AppStoreApplicationVersion, error)
	Update(appVersions []*AppStoreApplicationVersion) error
}

type AppStoreApplicationVersionRepositoryImpl struct {
	dbConnection *pg.DB
	Logger       *zap.SugaredLogger
}

func NewAppStoreApplicationVersionRepositoryImpl(Logger *zap.SugaredLogger, dbConnection *pg.DB) *AppStoreApplicationVersionRepositoryImpl {
	return &AppStoreApplicationVersionRepositoryImpl{dbConnection: dbConnection, Logger: Logger}
}

type AppStoreApplicationVersion struct {
	TableName   struct{}  `sql:"app_store_application_version" pg:",discard_unknown_columns"`
	Id          int       `sql:"id,pk"`
	Version     string    `sql:"version"`
	AppVersion  string    `sql:"app_version"`
	Created     time.Time `sql:"created"`
	Deprecated  bool      `sql:"deprecated,notnull"`
	Description string    `sql:"description"`
	Digest      string    `sql:"digest"`
	Icon        string    `sql:"icon"`
	Name        string    `sql:"name"`
	Source      string    `sql:"source"`
	Home        string    `sql:"home"`
	ValuesYaml  string    `sql:"values_yaml"`
	ChartYaml   string    `sql:"chart_yaml"`
	AppStoreId  int       `sql:"app_store_id"`
	AuditLog
	RawValues        string `sql:"raw_values"`
	Readme           string `sql:"readme"`
	ValuesSchemaJson string `sql:"values_schema_json"`
	Notes            string `sql:"notes"`
	AppStore         *AppStore
}

func (impl AppStoreApplicationVersionRepositoryImpl) FindVersionsByAppStoreId(appStoreId int) ([]*AppStoreApplicationVersion, error) {
	var appStoreApplicationVersion []*AppStoreApplicationVersion
	err := impl.dbConnection.Model(&appStoreApplicationVersion).
		Column("id", "version", "created").
		Where("app_store_id =?", appStoreId).
		Select()
	return appStoreApplicationVersion, err

}

func (impl AppStoreApplicationVersionRepositoryImpl) Save(versions *[]*AppStoreApplicationVersion) error {
	_, err := impl.dbConnection.Model(versions).OnConflict("DO NOTHING").Insert()
	return err
}

func (impl AppStoreApplicationVersionRepositoryImpl) FindOneByAppStoreIdAndVersion(appStoreId int, version string) (*AppStoreApplicationVersion, error) {
	appStoreApplicationVersion := &AppStoreApplicationVersion{}
	err := impl.dbConnection.Model(appStoreApplicationVersion).
		Where("app_store_id =?", appStoreId).
		Where("version =?", version).
		Limit(1).
		Select()
	return appStoreApplicationVersion, err
}

func (impl AppStoreApplicationVersionRepositoryImpl) Update(appVersions []*AppStoreApplicationVersion) error {
	err := impl.dbConnection.RunInTransaction(func(tx *pg.Tx) error {
		for _, version := range appVersions {
			_, err := tx.Model(version).WherePK().UpdateNotNull()
			if err != nil {
				return err
			}
		}
		return nil
	})
	return err
}
