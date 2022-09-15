package pkg

import (
	"bytes"
	"fmt"
	"github.com/devtron-labs/chart-sync/internal/sql"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
	"io"
	"net/http"
	"time"
)

type HelmRepoManager interface {
	LoadIndexFile(chartRepo *sql.ChartRepo) (*repo.IndexFile, error)
	ValuesJson(baseurl string, version *repo.ChartVersion) (rawValues string, readme string, valuesSchemaJson string, notes string, err error)
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

func (impl *HelmRepoManagerImpl) ValuesJson(baseurl string, version *repo.ChartVersion) (rawValues string, readme string, valuesSchemaJson string, notes string, err error) {
	absoluteChartURL, err := repo.ResolveReferenceURL(baseurl, version.URLs[0])
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to parse %s as URL: %v", baseurl, err)
	}
	/*httpGetter, err := getter.NewHTTPGetter(getter.WithURL(absoluteChartURL))
	if err != nil {
		return "", "", "", "", err
	}*/
	c, err := get(absoluteChartURL)
	if err != nil {
		fmt.Println("err", err)
		return "", "", "", "", err
	}
	c = bytes.NewBuffer(nil)
	if c == nil {
		return rawValues, readme, "", notes, err
	}
	/*chart, err := loader.LoadArchive(c)
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
	}*/

	return rawValues, readme, "", notes, err

	//return rawValues, readme, string(chart.Schema), notes, err
}

func get(href string) (*bytes.Buffer, error) {
	buf := bytes.NewBuffer(nil)

	// Set a helm specific user agent so that a repo server and metrics can
	// separate helm calls from other tools interacting with repos.
	req, err := http.NewRequest("GET", href, nil)
	if err != nil {
		return buf, err
	}

	transport := &http.Transport{
		DisableCompression: true,
		Proxy:              http.ProxyFromEnvironment,
	}
	transport.TLSClientConfig.InsecureSkipVerify = true

	client := &http.Client{
		Transport: transport,
		Timeout:   time.Duration(30) * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return buf, err
	}
	if resp.StatusCode != 200 {
		return buf, errors.Errorf("failed to fetch %s : %s", href, resp.Status)
	}

	_, err = io.Copy(buf, resp.Body)
	fmt.Println("DEBUGGING copied...")
	err = resp.Body.Close()
	if err != nil {
		fmt.Println("DEBUGGING error in closing")
	}
	fmt.Println("DEBUGGING closed...")
	return buf, err
}
