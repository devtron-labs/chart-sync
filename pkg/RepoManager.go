package pkg

import (
	"bytes"
	"fmt"
	"github.com/devtron-labs/chart-sync/internals/sql"
	"github.com/devtron-labs/chart-sync/util"
	registry2 "github.com/devtron-labs/common-lib/helmLib/registry"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/repo"
	"net/url"
	"path"
	"strings"
)

const (
	CERTIFICATE_FILE_PATH     = "/registry-credentials"
	INSECURE_CONNETION_STRING = "insecure"
	SECURE_WITH_CERT_STRING   = "secure-with-cert"
)

type HelmRepoManager interface {
	LoadIndexFile(chartRepo *sql.ChartRepo) (*repo.IndexFile, error)
	ValuesJson(repoUrl string, version *repo.ChartVersion, username string, password string, allowInsecureConnection bool) (rawValues string, readme string, valuesSchemaJson string, notes string, err error)
	OCIRepoValuesJson(client *registry.Client, registryUrl, chartName, version string) (chartData ChartData, err error)
	RegistryLogin(client *registry.Client, store *sql.DockerArtifactStore, username, password string) error
	FetchOCIChartTagsList(settings *registry2.Settings, ociRepoURL string) ([]string, error)
	LoadChartFromOCIRepo(client *registry.Client, registryUrl, chartName, version string) (*chart.Chart, string, error)
}

type HelmRepoManagerImpl struct {
	Logger   *zap.SugaredLogger
	Settings *cli.EnvSettings
}

func NewHelmRepoManagerImpl(logger *zap.SugaredLogger) *HelmRepoManagerImpl {
	return &HelmRepoManagerImpl{
		Logger:   logger,
		Settings: cli.New(),
	}
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
func (impl *HelmRepoManagerImpl) OCIRepoValuesJson(client *registry.Client, registryUrl, chartName, version string) (chartData ChartData, err error) {
	chart, digest, err := impl.LoadChartFromOCIRepo(client, registryUrl, chartName, version)
	if err != nil {
		return ChartData{}, err
	}
	// get values.yaml
	rawFiles := chart.Raw
	for _, f := range rawFiles {
		if strings.EqualFold(f.Name, "values.yaml") {
			chartData.RawValues = string(f.Data)
			break
		}
	}

	// get readme
	files := chart.Files
	for _, f := range files {
		if strings.EqualFold(f.Name, "README.md") {
			chartData.Readme = string(f.Data)
			break
		}
	}

	// get notes
	for _, templateFile := range chart.Templates {
		if strings.EqualFold(templateFile.Name, "NOTES.txt") {
			chartData.Notes = string(templateFile.Data)
			break
		}
	}
	chartData.MetaData = chart.Metadata
	chartData.ValuesSchemaJson = string(chart.Schema)
	chartData.Digest = digest

	return chartData, err
}

// FetchOCIChartTagsList list down all tags in of the given repository without pagination.
func (impl *HelmRepoManagerImpl) FetchOCIChartTagsList(settings *registry2.Settings, ociRepoURL string) ([]string, error) {
	// Retrieve list of repository tags
	client := settings.RegistryClient
	tags, err := client.FetchAllTags(strings.TrimPrefix(ociRepoURL, fmt.Sprintf("%s://", registry.OCIScheme)))
	if err != nil || len(tags) == 0 {
		if err != nil {
			err = fmt.Errorf("unable to locate any tags in provided repository: %s", ociRepoURL)
		}
		impl.Logger.Errorw("error in fetching repository tags, FetchOCIChartTagsList", "repo url", ociRepoURL, "err", err)
		return nil, err
	}
	return tags, nil
}

func (impl *HelmRepoManagerImpl) RegistryLogin(client *registry.Client, store *sql.DockerArtifactStore, username, password string) error {

	var loginOptions []registry.LoginOption
	loginOptions = append(loginOptions, registry.LoginOptBasicAuth(username, password))
	loginOptions = append(loginOptions, registry.LoginOptInsecure(store.Connection == INSECURE_CONNETION_STRING))
	if store.Connection == SECURE_WITH_CERT_STRING {
		certificateFilePath, err := registry2.CreateCertificateFile(store.Id, store.Cert)
		if err != nil {
			impl.Logger.Errorw("error in creating certificate file path for registry", "registryName", store.Id, "err", err)
			return err
		}
		loginOptions = append(loginOptions, registry.LoginOptTLSClientConfig("", "", certificateFilePath))
	}

	err := client.Login(store.RegistryURL,
		loginOptions...,
	)
	if err != nil {
		impl.Logger.Errorw("error in registry login, RegistryLogin", "DockerArtifactStoreId", store.Id, "err", err)
		return err
	}
	return nil
}

func (impl *HelmRepoManagerImpl) LoadChartFromOCIRepo(client *registry.Client, registryUrl, chartname, version string) (*chart.Chart, string, error) {
	ref := fmt.Sprintf("%s:%s",
		path.Join(TrimSchemeFromURL(registryUrl), chartname),
		version)
	chartDetails, err := client.Pull(
		ref,
		registry.PullOptWithChart(true),
		registry.PullOptWithProv(true),
		registry.PullOptIgnoreMissingProv(true),
	)
	if err != nil || chartDetails == nil || chartDetails.Chart == nil {
		if err == nil {
			err = fmt.Errorf("error in pulling chart from registry, ChartRepo: %s", ref)
		}
		impl.Logger.Errorw("error in pulling chart from registry, LoadChartFromOCIRepo", "chart repo", ref, "err", err)
		return nil, "", err
	}
	chart, err := loader.LoadArchive(bytes.NewBuffer(chartDetails.Chart.Data))
	if err != nil || chart == nil {
		if err == nil {
			err = fmt.Errorf("error in loading chart bytes, ChartRepo: %s", ref)
		}
		impl.Logger.Errorw("error in loading chart bytes, LoadChartFromOCIRepo", "chart repo", ref, "err", err)
		return nil, "", err
	}
	return chart, chartDetails.Chart.Digest, nil
}

func TrimSchemeFromURL(registryUrl string) string {
	parsedUrl, err := url.Parse(registryUrl)
	if err != nil {
		return registryUrl
	}
	urlWithoutScheme := parsedUrl.Host + parsedUrl.Path
	urlWithoutScheme = strings.TrimPrefix(urlWithoutScheme, "/")
	return urlWithoutScheme
}
