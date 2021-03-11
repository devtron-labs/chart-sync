package pkg

import (
	"fmt"
	"github.com/devtron-labs/chart-sync/internal/sql"
	"go.uber.org/zap"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/getter"
	helm_env "k8s.io/helm/pkg/helm/environment"
	"k8s.io/helm/pkg/repo"
	"path/filepath"
	"strings"
)

type HelmRepoManager interface {
	LoadIndexFile(chartRepo *sql.ChartRepo) (*repo.IndexFile, error)
	ValuesJson(baseurl string, version *repo.ChartVersion) (rawValues string, readme string, err error)
}
type HelmRepoManagerImpl struct {
	logger *zap.SugaredLogger
}

func NewHelmRepoManagerImpl(logger *zap.SugaredLogger) *HelmRepoManagerImpl {
	return &HelmRepoManagerImpl{logger: logger}
}

func (impl *HelmRepoManagerImpl) LoadIndexFile(chartRepo *sql.ChartRepo) (*repo.IndexFile, error) {
	helmRepoConfig := &repo.Entry{
		Name:     chartRepo.Name,
		Cache:    filepath.Join("/tmp", fmt.Sprintf("%s-index.yaml", chartRepo.Name)),
		URL:      chartRepo.Url,
		Username: chartRepo.Username,
		Password: chartRepo.Password,
		CertFile: chartRepo.CertFile,
		KeyFile:  chartRepo.KeyFile,
		CAFile:   chartRepo.CAFile,
	}
	helmRepo, err := repo.NewChartRepository(helmRepoConfig, getter.All(helm_env.EnvSettings{}))
	if err != nil {
		return nil, err
	}
	if err := helmRepo.DownloadIndexFile(""); err != nil {
		return nil, fmt.Errorf("Looks like %q is not a valid chart repository or cannot be reached: %s", chartRepo.Url, err.Error())
	}
	index, err := repo.LoadIndexFile(helmRepoConfig.Cache)
	if err != nil {
		return nil, err
	}
	index.SortEntries()
	return index, nil
}

func (impl *HelmRepoManagerImpl) ValuesJson(baseurl string, version *repo.ChartVersion) (rawValues string, readme string, err error) {
	absoluteChartURL, err := repo.ResolveReferenceURL(baseurl, version.URLs[0])
	if err != nil {
		return "", "", fmt.Errorf("failed to parse %s as URL: %v", baseurl, err)
	}
	httpGetter, err := getter.NewHTTPGetter(absoluteChartURL, "", "", "")
	if err != nil {
		return "", "", err
	}
	c, err := httpGetter.Get(absoluteChartURL)
	if err != nil {
		fmt.Println("err", err)
		return "", "", err
	}
	chart, err := chartutil.LoadArchive(c)
	if err != nil {
		fmt.Println("err", err)
		return "", "", err
	}
	val := chart.GetValues()
	yamlValues := val.Raw

	readme = ""
	files := chart.GetFiles()
	for _, f := range files {
		if strings.EqualFold(f.TypeUrl, "README.md") {
			readme = string(f.GetValue())
		}
	}

	return yamlValues, readme, err
}
