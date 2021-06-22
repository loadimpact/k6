/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2021 Load Impact
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

package stats

import (
	"context"
	"fmt"
	"sync"
)

// Registry is what can create metrics
// TODO maybe rename the whole package to metrics so we have `metrics.Registry` ?
type Registry struct {
	metrics map[string]*Metric
	sink    chan<- SampleContainer
	l       sync.RWMutex
	// TODO maybe "seal" it after we no longer want to get new metrics?
}

func NewRegistry(sink chan<- SampleContainer) *Registry {
	return &Registry{
		metrics: make(map[string]*Metric),
		sink:    sink,
	}
}

// NewMetric returns new metric registered to this registry
// TODO have multiple versions returning specific metric types when we have such things
func (r *Registry) NewMetric(name string, typ MetricType, t ...ValueType) (*Metric, error) {
	r.l.Lock()
	defer r.l.Unlock()
	oldMetric, ok := r.metrics[name]

	if !ok {
		m := New(name, typ, t...)
		m.r = r
		r.metrics[name] = m
		return m, nil
	}
	// Name is checked. TODO check Contains as well
	if oldMetric.Type != typ {
		return nil, fmt.Errorf("Metric `%s` already exist but with type %s", name, oldMetric.Type)
	}
	return oldMetric, nil
}

func (r *Registry) MustNewMetric(name string, typ MetricType, t ...ValueType) *Metric {
	m, err := r.NewMetric(name, typ, t...)
	if err != nil {
		panic(err)
	}
	return m
}

// PushContainer emits a contained through the registry's sink
// TODO drop this somehow
func (r *Registry) PushContainer(ctx context.Context, container SampleContainer) {
	PushIfNotDone(ctx, r.sink, container)
}

func WithRegistry(ctx context.Context, r *Registry) context.Context {
	return context.WithValue(ctx, registryKey, r)
}

func GetRegistry(ctx context.Context) *Registry {
	return ctx.Value(registryKey).(*Registry)
}

type contextKey uint64

const (
	registryKey contextKey = iota
)
