package pkg

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/devtron-labs/chart-sync/internal/sql"
	"github.com/devtron-labs/chart-sync/util"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/repo"
	"path"
	"strings"
)

const (
	INSECURE_CONNETION_STRING = "insecure"
)

type HelmRepoManager interface {
	LoadIndexFile(chartRepo *sql.ChartRepo) (*repo.IndexFile, error)
	ValuesJson(repoUrl string, version *repo.ChartVersion, username string, password string, allowInsecureConnection bool) (rawValues string, readme string, valuesSchemaJson string, notes string, err error)
	OCIRepoValuesJson(client *registry.Client, registryUrl, chartName, version string) (metaData *chart.Metadata, rawValues, readme, valuesSchemaJson, notes, diagest string, err error)
	RegistryLogin(client *registry.Client, store *sql.DockerArtifactStore, username, password string) error
	ExtractCredentialsForRegistry(registryCredential *sql.DockerArtifactStore) (string, string, error)
	FetchOCIChartTagsList(client *registry.Client, ociRepoURL string) ([]string, error)
	LoadChartFromOCIRepo(client *registry.Client, registryUrl, chartName, version string) (*chart.Chart, string, error)
}

type HelmRepoManagerImpl struct {
	logger   *zap.SugaredLogger
	settings *cli.EnvSettings
}

func NewHelmRepoManagerImpl(logger *zap.SugaredLogger) *HelmRepoManagerImpl {
	return &HelmRepoManagerImpl{
		logger:   logger,
		settings: cli.New(),
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
func (impl *HelmRepoManagerImpl) OCIRepoValuesJson(client *registry.Client, registryUrl, chartName, version string) (metaData *chart.Metadata, rawValues, readme, valuesSchemaJson, notes, diagest string, err error) {
	chart, diagest, err := impl.LoadChartFromOCIRepo(client, registryUrl, chartName, version)
	if err != nil {
		return nil, "", "", "", "", "", err
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

	return chart.Metadata, rawValues, readme, string(chart.Schema), notes, diagest, err
}

// FetchOCIChartTagsList list down all tags in of the given repository without pagination.
func (impl *HelmRepoManagerImpl) FetchOCIChartTagsList(client *registry.Client, ociRepoURL string) ([]string, error) {
	// Retrieve list of repository tags
	tags, err := client.FetchAllTags(strings.TrimPrefix(ociRepoURL, fmt.Sprintf("%s://", registry.OCIScheme)))
	if err != nil || len(tags) == 0 {
		if err != nil {
			err = fmt.Errorf("unable to locate any tags in provided repository: %s", ociRepoURL)
		}
		impl.logger.Errorw("error in fetching repository tags, FetchOCIChartTagsList", "repo url", ociRepoURL, "err", err)
		return nil, err
	}
	return tags, nil
}

func (impl *HelmRepoManagerImpl) RegistryLogin(client *registry.Client, store *sql.DockerArtifactStore, username, password string) error {
	err := client.Login(store.RegistryURL,
		registry.LoginOptBasicAuth(username, password),
		registry.LoginOptInsecure(store.Connection == INSECURE_CONNETION_STRING),
		registry.LoginOptTLSClientConfig(store.Cert, "", ""),
	)
	if err != nil {
		impl.logger.Errorw("error in registry login, RegistryLogin", "DockerArtifactStoreId", store.Id, "err", err)
		return err
	}
	return nil
}

func (impl *HelmRepoManagerImpl) ExtractCredentialsForRegistry(registryCredential *sql.DockerArtifactStore) (string, string, error) {
	username := registryCredential.Username
	pwd := registryCredential.Password
	if registryCredential.RegistryType == sql.REGISTRYTYPE_ECR {
		accessKey, secretKey := registryCredential.AWSAccessKeyId, registryCredential.AWSSecretAccessKey
		var creds *credentials.Credentials

		if len(accessKey) == 0 || len(secretKey) == 0 {
			sess, err := session.NewSession(&aws.Config{
				Region: &registryCredential.AWSRegion,
			})
			if err != nil {
				return "", "", err
			}
			creds = ec2rolecreds.NewCredentials(sess)
		} else {
			creds = credentials.NewStaticCredentials(accessKey, secretKey, "")
		}
		sess, err := session.NewSession(&aws.Config{
			Region:      &registryCredential.AWSRegion,
			Credentials: creds,
		})
		if err != nil {
			return "", "", err
		}
		svc := ecr.New(sess)
		input := &ecr.GetAuthorizationTokenInput{}
		authData, err := svc.GetAuthorizationToken(input)
		if err != nil {
			return "", "", err
		}
		// decode token
		token := authData.AuthorizationData[0].AuthorizationToken
		decodedToken, err := base64.StdEncoding.DecodeString(*token)
		if err != nil {
			return "", "", err
		}
		credsSlice := strings.Split(string(decodedToken), ":")
		username = credsSlice[0]
		pwd = credsSlice[1]

	}
	return username, pwd, nil
}

func (impl *HelmRepoManagerImpl) LoadChartFromOCIRepo(client *registry.Client, registryUrl, chartname, version string) (*chart.Chart, string, error) {
	ref := fmt.Sprintf("%s:%s",
		path.Join(strings.TrimPrefix(registryUrl, fmt.Sprintf("%s://", registry.OCIScheme)), chartname),
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
		impl.logger.Errorw("error in pulling chart from registry, LoadChartFromOCIRepo", "chart repo", ref, "err", err)
		return nil, "", err
	}
	chart, err := loader.LoadArchive(bytes.NewBuffer(chartDetails.Chart.Data))
	if err != nil || chart == nil {
		if err == nil {
			err = fmt.Errorf("error in loading chart bytes, ChartRepo: %s", ref)
		}
		impl.logger.Errorw("error in loading chart bytes, LoadChartFromOCIRepo", "chart repo", ref, "err", err)
		return nil, "", err
	}
	return chart, chartDetails.Chart.Digest, nil
}
