package registry // import "helm.sh/helm/v3/pkg/registry"
import (
	"oras.land/oras-go/pkg/registry"
	registryremote "oras.land/oras-go/pkg/registry/remote"
	"strings"
)

// FetchAllTags implements Tags function but removes semver StrictNewVersion check
// fix for issue https://github.com/devtron-labs/devtron/issues/4385, tags were not semver compatible, so they were getting filtered by StrictNewVersion check
func (c *Client) FetchAllTags(ref string) ([]string, error) {
	parsedReference, err := registry.ParseReference(ref)
	if err != nil {
		return nil, err
	}

	repository := registryremote.Repository{
		Reference: parsedReference,
		Client:    c.registryAuthorizer,
	}

	var registryTags []string

	for {
		registryTags, err = registry.Tags(ctx(c.out, c.debug), &repository)
		if err != nil {
			// Fallback to http based request
			if !repository.PlainHTTP && strings.Contains(err.Error(), "server gave HTTP response") {
				repository.PlainHTTP = true
				continue
			}
			return nil, err
		}

		break

	}
	return registryTags, nil
}
