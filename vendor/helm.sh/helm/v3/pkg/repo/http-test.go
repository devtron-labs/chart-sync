package repo

//import (
//
//	"fmt"
//	"net/url"
//	"path"
//	"helm.sh/helm/v3/pkg/getter"
//	"io/ioutil"
//
//)
//func main(){
//	URL:= "https://raw.githubusercontent.com/pawan-59/private-chart/main"
//	parsedURL2, err := url.Parse(URL)
//	if err != nil {
//		fmt.Printf(" ")
//	}
//	parsedURL2.RawPath = path.Join(parsedURL2.RawPath, "index.yaml")
//	parsedURL2.Path = path.Join(parsedURL2.Path, "index.yaml")
//	indexURL2 := parsedURL2.String()
//	resp, err := getter.Getter.Get(indexURL2,
//		getter.WithURL(URL),
//		getter.WithInsecureSkipVerifyTLS(false),
//		getter.WithTLSClientConfig("", "", ""),
//		getter.WithBasicAuth("pawan-59", "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"),
//	)
//	if err != nil {
//		fmt.Printf(" ")
//	}
//	index,err := ioutil.ReadAll(resp)
//	fmt.Printf(string(index))
//}
//
