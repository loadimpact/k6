package kafka

import (
	"encoding/json"

	"github.com/loadimpact/k6/stats"
	"github.com/loadimpact/k6/stats/influxdb"
	jsonCollector "github.com/loadimpact/k6/stats/json"
)

func formatSamples(format string, samples []stats.Sample) ([]string, error) {
	var metrics []string

	switch format {
	case "influx":
		i, err := influxdb.New(influxdb.Config{})
		if err != nil {
			return nil, err
		}

		metrics, err = i.Format(samples)
		if err != nil {
			return nil, err
		}
	default:
		for _, sample := range samples {
			env := jsonCollector.WrapSample(&sample)
			metric, err := json.Marshal(env)
			if err != nil {
				return nil, err
			}

			metrics = append(metrics, string(metric))
		}
	}

	return metrics, nil
}
