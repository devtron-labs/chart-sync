package main

import (
	"fmt"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/getter"
	helm_env "k8s.io/helm/pkg/helm/environment"
	"k8s.io/helm/pkg/helm/helmpath"
	"k8s.io/helm/pkg/repo"
	"strings"
)

const (
	stableRepository                  = "stable"
	stableRepositoryURL               = "https://kubernetes-charts.storage.googleapis.com"
	home                helmpath.Home = "helm"
	cache                             = "/tmp/stable-index.yaml"
)

func main1() {
	fmt.Println("hello")
	err := loadRepo()
	fmt.Println(err)
}

func loadRepo() error {

	c := repo.Entry{
		Name:  stableRepository,
		URL:   stableRepositoryURL,
		Cache: cache, //home.CacheIndex(stableRepository),
	}
	r, err := repo.NewChartRepository(&c, getter.All(helm_env.EnvSettings{}))
	if err != nil {
		return err
	}

	// In this case, the cacheFile is always absolute. So passing empty string
	// is safe.
	if err := r.DownloadIndexFile(""); err != nil {
		return fmt.Errorf("Looks like %q is not a valid chart repository or cannot be reached: %s", stableRepositoryURL, err.Error())
	}
	index, err := repo.LoadIndexFile(cache)
	if err != nil {
		return err
	}
	index.SortEntries()
	entries := index.Entries
	for app, versions := range entries {
		for _, v := range versions {
			c, err := r.Client.Get(v.URLs[0])
			if err != nil {
				fmt.Println("err", err)
				continue
			}
			chart, err := chartutil.LoadArchive(c)
			if err != nil {
				fmt.Println("err", err)
			}
			val := chart.GetValues()
			fmt.Println(val.Values)
			fmt.Println(val.String())
			files := chart.GetFiles()
			for _, f := range files {
				if strings.EqualFold(f.TypeUrl, "README.md") {
					readme := string(f.GetValue())
					fmt.Println(readme)
				}
			}
			fmt.Println(files)
			if len(v.URLs) > 1 {
				fmt.Println(app)
				fmt.Println("found")

			}
		}

		/*fmt.Println(len(versions))
		url:=versions[0].URLs[0]*/

	}
	/*meta, err:=	chartutil.LoadChartfile("/Users/nishant/.helm/repository/cache/stable-index.yaml")
	if err!=nil{
		println(err)
	}else {
		fmt.Println(len(meta))
	}*/
	return nil
}
