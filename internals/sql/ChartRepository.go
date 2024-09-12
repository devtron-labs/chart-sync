package sql

import (
	"github.com/go-pg/pg"
)

//---------------------------chart repository------------------

type ChartRepo struct {
	tableName               struct{} `sql:"chart_repo"`
	Id                      int      `sql:"id,pk"`
	Name                    string   `sql:"name"`
	Url                     string   `sql:"url"`
	Active                  bool     `sql:"active"`
	Deleted                 bool     `sql:"deleted"`
	Default                 bool     `sql:"is_default"`
	External                bool     `sql:"external"`
	Username                string   `sql:"user_name"`
	Password                string   `sql:"password"`
	CertFile                string   `sql:"-"`
	KeyFile                 string   `sql:"-"`
	CAFile                  string   `sql:"-"`
	AllowInsecureConnection bool     `sql:"allow_insecure_connection"`
	AuditLog
}

type ChartRepoRepository interface {
	Save(chartRepo *ChartRepo) error
	GetDefault() (*ChartRepo, error)
	FindById(id int) (*ChartRepo, error)
	GetAll() (repos []*ChartRepo, err error)
}
type ChartRepoRepositoryImpl struct {
	dbConnection *pg.DB
}

func NewChartRepoRepositoryImpl(dbConnection *pg.DB) *ChartRepoRepositoryImpl {
	return &ChartRepoRepositoryImpl{
		dbConnection: dbConnection,
	}
}

func (impl ChartRepoRepositoryImpl) Save(chartRepo *ChartRepo) error {
	return impl.dbConnection.Insert(chartRepo)
}
func (impl ChartRepoRepositoryImpl) GetDefault() (*ChartRepo, error) {
	repo := &ChartRepo{}
	err := impl.dbConnection.Model(repo).
		Where("is_default = ?", true).
		Where("deleted = ?", false).
		Where("active = ?", true).Select()
	return repo, err
}

func (impl ChartRepoRepositoryImpl) FindById(id int) (*ChartRepo, error) {
	repo := &ChartRepo{}
	err := impl.dbConnection.Model(repo).
		Where("id = ?", id).
		Where("deleted = ?", false).
		Select()
	return repo, err
}

func (impl *ChartRepoRepositoryImpl) GetAll() (repos []*ChartRepo, err error) {
	err = impl.dbConnection.Model(&repos).
		Where("external = ?", true).
		Where("deleted = ?", false).
		Select()
	return repos, err
}


