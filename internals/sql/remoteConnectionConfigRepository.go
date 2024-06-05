package sql

import (
	"github.com/go-pg/pg"
	"go.uber.org/zap"
)

type RemoteConnectionConfig struct {
	tableName        struct{} `sql:"remote_connection_config" pg:",discard_unknown_columns"`
	Id               int      `sql:"id,pk"`
	ConnectionMethod string   `sql:"connection_method"`
	ProxyUrl         string   `sql:"proxy_url"`
	SSHServerAddress string   `sql:"ssh_server_address"`
	SSHUsername      string   `sql:"ssh_username"`
	SSHPassword      string   `sql:"ssh_password"`
	SSHAuthKey       string   `sql:"ssh_auth_key"`
	Deleted          bool     `sql:"deleted,notnull"`
	AuditLog
}

type RemoteConnectionRepository interface {
	GetById(id int) (*RemoteConnectionConfig, error)
}

type RemoteConnectionRepositoryImpl struct {
	logger       *zap.SugaredLogger
	dbConnection *pg.DB
}

func NewRemoteConnectionRepositoryImpl(dbConnection *pg.DB, logger *zap.SugaredLogger) *RemoteConnectionRepositoryImpl {
	return &RemoteConnectionRepositoryImpl{
		logger:       logger,
		dbConnection: dbConnection,
	}
}

func (repo *RemoteConnectionRepositoryImpl) GetById(id int) (*RemoteConnectionConfig, error) {
	model := &RemoteConnectionConfig{}
	err := repo.dbConnection.Model(model).
		Where("id = ?", id).
		Where("deleted = ?", false).
		Select()
	if err != nil && err != pg.ErrNoRows {
		repo.logger.Errorw("error in getting remote connection config", "err", err, "id", id)
		return nil, err
	}
	return model, nil
}
