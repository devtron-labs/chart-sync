package registry // import "helm.sh/helm/v3/pkg/registry"
import (
	"oras.land/oras-go/pkg/registry"
	registryremote "oras.land/oras-go/pkg/registry/remote"
)

// GetTagsIgnoreSemVer implements Tags function but removes semver StrictNewVersion check
// fix for issue https://github.com/devtron-labs/devtron/issues/4385, tags were not semver compatible, so they were getting filtered by StrictNewVersion check
func (c *Client) GetTagsIgnoreSemVer(ref string) ([]string, error) {
	parsedReference, err := registry.ParseReference(ref)
	if err != nil {
		return nil, err
	}

	repository := registryremote.Repository{
		Reference: parsedReference,
		Client:    c.registryAuthorizer,
		PlainHTTP: c.plainHTTP,
	}

	var registryTags []string
	registryTags, err = registry.Tags(ctx(c.out, c.debug), &repository)
	if err != nil {
		return nil, err
	}

	return registryTags, nil
}
