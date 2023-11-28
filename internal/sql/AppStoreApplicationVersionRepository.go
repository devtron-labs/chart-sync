package sql

import (
	"github.com/go-pg/pg"
	"go.uber.org/zap"
	"time"
)

type AppStoreApplicationVersionRepository interface {
	FindVersionsByAppStoreId(appStoreId int) ([]*AppStoreApplicationVersion, error)
	Save(versions *[]*AppStoreApplicationVersion) (isNewChartVersionFound bool, err error)
	FindLatestCreated(appStoreId int) (*AppStoreApplicationVersion, error)
	FindOneByAppStoreIdAndVersion(appStoreId int, version string) (*AppStoreApplicationVersion, error)
	FindLatest(appStoreId int) (*AppStoreApplicationVersion, error)
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
	Latest      bool      `sql:"latest,notnull"`
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

func (impl AppStoreApplicationVersionRepositoryImpl) Save(versions *[]*AppStoreApplicationVersion) (isNewChartVersionFound bool, err error) {
	//for _, version := range *versions {
	//query := "WITH upsert AS (UPDATE app_store_application_version SET latest=false where app_store_id=? and version=? returning * ) INSERT INTO app_store_application_version (version, description, app_version, digest, home,deprecated, name,values_yaml,chart_yaml,latest,app_store_id,created_on,updated_on,created_by,updated_by, raw_values,readme,values_schema_json,notes) SELECT ?,?,?,?,?,?,?,?,?,?,?,now(),now(),1,1,?,?,?,? WHERE NOT EXISTS (SELECT * FROM upsert)"
	//res, err := impl.dbConnection.Exec(query, version.AppStoreId, version.Version, version.Version, version.Description, version.AppVersion, version.Digest, version.Home, version.Deprecated, version.Name, version.ValuesYaml, version.ChartYaml, false, version.AppStoreId, version.RawValues, version.Readme, version.ValuesSchemaJson, version.Notes)

	//}
	res, err := impl.dbConnection.Model(versions).Insert()
	if err != nil {
		impl.Logger.Errorw("error in creating app store application version for app store", "err", err)
		return false, err
	}
	if res.RowsAffected() > 0 {
		isNewChartVersionFound = true
	}
	return isNewChartVersionFound, nil
}

func (impl AppStoreApplicationVersionRepositoryImpl) FindLatestCreated(appStoreId int) (*AppStoreApplicationVersion, error) {
	appStoreApplicationVersion := &AppStoreApplicationVersion{}
	err := impl.dbConnection.Model(appStoreApplicationVersion).
		Where("app_store_id =?", appStoreId).
		Order("created DESC").Limit(1).
		Select()
	return appStoreApplicationVersion, err
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

func (impl AppStoreApplicationVersionRepositoryImpl) FindLatest(appStoreId int) (*AppStoreApplicationVersion, error) {
	appStoreApplicationVersion := &AppStoreApplicationVersion{}
	err := impl.dbConnection.Model(appStoreApplicationVersion).
		Where("app_store_id =?", appStoreId).
		Where("latest =?", true).
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
