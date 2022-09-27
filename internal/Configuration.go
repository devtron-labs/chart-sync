package internal

import "github.com/caarlos0/env"

type Configuration struct {
	AppStoreAppVersionsSaveChunkSize int `env:"APP_STORE_APPLICATION_VERSIONS_SAVE_CHUNK_SIZE" envDefault:"20"`
}

func ParseConfiguration() (*Configuration, error) {
	cfg := &Configuration{}
	err := env.Parse(cfg)
	return cfg, err
}
