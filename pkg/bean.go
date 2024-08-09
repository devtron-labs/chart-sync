package pkg

import "helm.sh/helm/v3/pkg/chart"

type ChartData struct {
	MetaData                                           *chart.Metadata
	RawValues, Readme, ValuesSchemaJson, Notes, Digest string
}
