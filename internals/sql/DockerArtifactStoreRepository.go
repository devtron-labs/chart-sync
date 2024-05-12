/*
 * Copyright (c) 2020 Devtron Labs
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package sql

import (
	"github.com/go-pg/pg"
	"github.com/go-pg/pg/orm"
)

const (
	REGISTRYTYPE_ECR                         = "ecr"
	REGISTRYTYPE_GCR                         = "gcr"
	REGISTRYTYPE_ARTIFACT_REGISTRY           = "artifact-registry"
	REGISTRYTYPE_OTHER                       = "other"
	REGISTRYTYPE_DOCKER_HUB                  = "docker-hub"
	JSON_KEY_USERNAME                 string = "_json_key"
	STORAGE_ACTION_TYPE_PULL                 = "PULL"
	STORAGE_ACTION_TYPE_PUSH                 = "PUSH"
	STORAGE_ACTION_TYPE_PULL_AND_PUSH        = "PULL/PUSH"
	OCI_REGISRTY_REPO_TYPE_CONTAINER         = "CONTAINER"
	OCI_REGISRTY_REPO_TYPE_CHART             = "CHART"
)

type RegistryType string

var OCI_REGISRTY_REPO_TYPE_LIST = []string{OCI_REGISRTY_REPO_TYPE_CONTAINER, OCI_REGISRTY_REPO_TYPE_CHART}

type DockerArtifactStore struct {
	tableName                struct{}     `sql:"docker_artifact_store" json:",omitempty"  pg:",discard_unknown_columns"`
	Id                       string       `sql:"id,pk" json:"id,,omitempty"`
	PluginId                 string       `sql:"plugin_id,notnull" json:"pluginId,omitempty"`
	RegistryURL              string       `sql:"registry_url" json:"registryUrl,omitempty"`
	RegistryType             RegistryType `sql:"registry_type,notnull" json:"registryType,omitempty"`
	IsOCICompliantRegistry   bool         `sql:"is_oci_compliant_registry,notnull" json:"isOCICompliantRegistry,omitempty"`
	AWSAccessKeyId           string       `sql:"aws_accesskey_id" json:"awsAccessKeyId,omitempty" `
	AWSSecretAccessKey       string       `sql:"aws_secret_accesskey" json:"awsSecretAccessKey,omitempty"`
	AWSRegion                string       `sql:"aws_region" json:"awsRegion,omitempty"`
	Username                 string       `sql:"username" json:"username,omitempty"`
	Password                 string       `sql:"password" json:"password,omitempty"`
	IsDefault                bool         `sql:"is_default,notnull" json:"isDefault"`
	Connection               string       `sql:"connection" json:"connection,omitempty"`
	Cert                     string       `sql:"cert" json:"cert,omitempty"`
	Active                   bool         `sql:"active,notnull" json:"active"`
	RemoteConnectionConfigId int          `sql:"remote_connection_config_id"`
	OCIRegistryConfig        []*OCIRegistryConfig
	RemoteConnectionConfig   *RemoteConnectionConfig
	AuditLog
}

type DockerArtifactStoreRepository interface {
	FindAllChartProviders() ([]*DockerArtifactStore, error)
	FindOne(storeId string) (*DockerArtifactStore, error)
}
type DockerArtifactStoreRepositoryImpl struct {
	dbConnection *pg.DB
}

func NewDockerArtifactStoreRepositoryImpl(dbConnection *pg.DB) *DockerArtifactStoreRepositoryImpl {
	return &DockerArtifactStoreRepositoryImpl{dbConnection: dbConnection}
}

func (impl DockerArtifactStoreRepositoryImpl) FindAllChartProviders() ([]*DockerArtifactStore, error) {
	var providers []*DockerArtifactStore
	err := impl.dbConnection.Model(&providers).
		Column("docker_artifact_store.*", "OCIRegistryConfig", "RemoteConnectionConfig").
		Where("active = ?", true).
		Relation("OCIRegistryConfig", func(q *orm.Query) (query *orm.Query, err error) {
			return q.Where("deleted IS FALSE and " +
				"is_chart_pull_active IS TRUE and repository_type='CHART' and " +
				"(repository_action='PULL' or repository_action='PULL/PUSH')"), nil
		}).
		Select()
	return providers, err
}

func (impl DockerArtifactStoreRepositoryImpl) FindOne(storeId string) (*DockerArtifactStore, error) {
	var provider DockerArtifactStore
	err := impl.dbConnection.Model(&provider).
		Column("docker_artifact_store.*", "OCIRegistryConfig", "RemoteConnectionConfig").
		Where("docker_artifact_store.id = ?", storeId).
		Where("active = ?", true).
		Relation("OCIRegistryConfig", func(q *orm.Query) (query *orm.Query, err error) {
			return q.Where("deleted IS FALSE and " +
				"is_chart_pull_active IS TRUE and repository_type='CHART' and " +
				"(repository_action='PULL' or repository_action='PULL/PUSH')"), nil
		}).
		Select()
	return &provider, err
}
