package pkg

import (
	"fmt"
	"github.com/devtron-labs/chart-sync/internal/sql"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"

	"strings"
)

type HelmRepoManager interface {
	LoadIndexFile(chartRepo *sql.ChartRepo) (*repo.IndexFile, error)
	ValuesJson(baseurl string, version *repo.ChartVersion) (rawValues string, readme string, schemaJson string, err error)
}
type HelmRepoManagerImpl struct {
	logger *zap.SugaredLogger
}

func NewHelmRepoManagerImpl(logger *zap.SugaredLogger) *HelmRepoManagerImpl {
	return &HelmRepoManagerImpl{logger: logger}
}

func (impl *HelmRepoManagerImpl) LoadIndexFile(chartRepo *sql.ChartRepo) (*repo.IndexFile, error) {
	helmRepoConfig := &repo.Entry{
		Name:                  chartRepo.Name,
		URL:                   chartRepo.Url,
		Username:              chartRepo.Username,
		Password:              chartRepo.Password,
		CertFile:              chartRepo.CertFile,
		KeyFile:               chartRepo.KeyFile,
		CAFile:                chartRepo.CAFile,
		InsecureSkipTLSverify: false,
	}
	helmRepo, err := repo.NewChartRepository(helmRepoConfig, getter.All(&cli.EnvSettings{}))

	if err != nil {
		return nil, err
	}
	indexfilelocation, err := helmRepo.DownloadIndexFile()
	if err != nil {
		return nil, fmt.Errorf("Looks like %q is not a valid chart repository or cannot be reached: %s", chartRepo.Url, err.Error())
	}
	index, err := repo.LoadIndexFile(indexfilelocation)
	if err != nil {
		return nil, err
	}
	index.SortEntries()
	return index, nil
}

func (impl *HelmRepoManagerImpl) ValuesJson(baseurl string, version *repo.ChartVersion) (rawValues string, readme string, schemaJson string, err error) {
	absoluteChartURL, err := repo.ResolveReferenceURL(baseurl, version.URLs[0])
	if err != nil {
		return "", "", "", fmt.Errorf("failed to parse %s as URL: %v", baseurl, err)
	}
	httpGetter, err := getter.NewHTTPGetter(getter.WithURL(absoluteChartURL))
	if err != nil {
		return "", "", "", err
	}
	c, err := httpGetter.Get(absoluteChartURL)
	if err != nil {
		fmt.Println("err", err)
		return "", "", "", err
	}
	chart, err := loader.LoadArchive(c)
	if err != nil {
		fmt.Println("err", err)
		return "", "", "", err
	}

	rawFiles := chart.Raw
	for _, f := range rawFiles {
		if strings.EqualFold(f.Name, "values.yaml") {
			rawValues = string(f.Data)
			break
		}
	}
	files := chart.Files
	for _, f := range files {
		fmt.Println("testing file name ", f.Name)
		if strings.EqualFold(f.Name, "README.md") {
			readme = string(f.Data)
			break
		}

		if strings.EqualFold(f.Name, "schema.json") {
			schemaJson = string(f.Data)
			break
		}
	}

	return rawValues, readme, schemaJson, err
}
