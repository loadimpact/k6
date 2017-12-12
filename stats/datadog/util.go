/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package datadog

import (
	"fmt"

	"github.com/DataDog/datadog-go/statsd"
	"github.com/loadimpact/k6/stats"
	log "github.com/sirupsen/logrus"
)

// MakeClient creates a new statsd buffered client
func MakeClient(conf Config) (*statsd.Client, error) {
	log.Info("Connecting to StatsD metrics server")
	connStr := fmt.Sprintf("%s:%s", conf.Addr, conf.Port)

	c, err := statsd.NewBuffered(connStr, conf.BufferSize)
	if err != nil {
		log.Info(err)
		return nil, err
	}
	return c, nil
}

func generateDataPoint(sample stats.Sample) *Sample {
	return &Sample{
		Type:   sample.Metric.Type,
		Metric: sample.Metric.Name,
		Data: SampleData{
			Time:  sample.Time,
			Value: sample.Value,
			Tags:  sample.Tags,
		},
	}
}
