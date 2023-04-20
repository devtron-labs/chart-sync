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
	ValuesJson(repoUrl string, version *repo.ChartVersion, username string, password string, allowInsecureConnection bool) (rawValues string, readme string, valuesSchemaJson string, notes string, err error)
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
		InsecureSkipTLSverify: chartRepo.AllowInsecureConnection,
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

func (impl *HelmRepoManagerImpl) ValuesJson(repoUrl string, version *repo.ChartVersion, username string, password string, allowInsecureConnection bool) (rawValues string, readme string, valuesSchemaJson string, notes string, err error) {
	absoluteChartURL, err := repo.ResolveReferenceURL(repoUrl, version.URLs[0])
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to parse %s as URL: %v", repoUrl, err)
	}

	var byteBuffer *bytes.Buffer
	if len(username) > 0 && len(password) > 0 {
		byteBuffer, err = util.GetFromPrivateUrlWithRetry(repoUrl, absoluteChartURL, username, password, allowInsecureConnection)
	} else {
		byteBuffer, err = util.GetFromPublicUrlWithRetry(absoluteChartURL)
	}

	if err != nil {
		fmt.Println("err", err)
		return "", "", "", "", err
	}
	chart, err := loader.LoadArchive(byteBuffer)
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
