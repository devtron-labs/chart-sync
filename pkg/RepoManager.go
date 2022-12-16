package pkg

import (
	"bytes"
	"fmt"
	"github.com/devtron-labs/chart-sync/internal/sql"
	"github.com/devtron-labs/chart-sync/util"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"

	"strings"
)

type HelmRepoManager interface {
	LoadIndexFile(chartRepo *sql.ChartRepo) (*repo.IndexFile, error)
	ValuesJson(baseurl string, version *repo.ChartVersion, username string, password string) (rawValues string, readme string, valuesSchemaJson string, notes string, err error)
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

func (impl *HelmRepoManagerImpl) ValuesJson(baseurl string, version *repo.ChartVersion, username string, password string) (rawValues string, readme string, valuesSchemaJson string, notes string, err error) {
	absoluteChartURL, err := repo.ResolveReferenceURL(baseurl, version.URLs[0])
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to parse %s as URL: %v", baseurl, err)
	}
	fmt.Printf("prakash:- going to fetch the charts from charts url inside index.yaml")
	byteArr, err := util.ReadFromUrlWithRetry(baseurl, absoluteChartURL, username, password)
	if err != nil {
		fmt.Println("err", err)
		return "", "", "", "", err
	}
	c := bytes.NewBuffer(byteArr)
	chart, err := loader.LoadArchive(c)
	if err != nil {
		fmt.Println("err", err)
		return "", "", "", "", err
	}

	// get values.yaml
	rawFiles := chart.Raw
	for _, f := range rawFiles {
		if strings.EqualFold(f.Name, "values.yaml") {
			rawValues = string(f.Data)
			break
		}
	}

	// get readme
	files := chart.Files
	for _, f := range files {
		if strings.EqualFold(f.Name, "README.md") {
			readme = string(f.Data)
			break
		}
	}

	// get notes
	for _, templateFile := range chart.Templates {
		if strings.EqualFold(templateFile.Name, "NOTES.txt") {
			notes = string(templateFile.Data)
			break
		}
	}

	return rawValues, readme, string(chart.Schema), notes, err
}
