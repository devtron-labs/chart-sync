package internals

import "github.com/caarlos0/env"

type Configuration struct {
	AppStoreAppVersionsSaveChunkSize int    `env:"APP_STORE_APPLICATION_VERSIONS_SAVE_CHUNK_SIZE" envDefault:"20"`
	ChartProviderId                  string `env:"CHART_PROVIDER_ID" envDefault:"*"` // * is used to sync all chart providers; else CHART_PROVIDER_ID should contain chart_repo_id OR docker_artifact_store_id
	IsOCIRegistry                    bool   `env:"IS_OCI_REGISTRY" envDefault:"true"`
	ParallelismLimitForTagProcessing int    `env:"PARALLELISM_LIMIT_FOR_TAG_PROCESSING" envDefault:"0"`
}

func ParseConfiguration() (*Configuration, error) {
	cfg := &Configuration{}
	err := env.Parse(cfg)
	return cfg, err
}
