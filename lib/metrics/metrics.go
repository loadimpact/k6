/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2017 Load Impact
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

package metrics

import (
	"context"

	"go.k6.io/k6/stats"
)

type BuiltInMetrics struct {
	VUs               *stats.Metric
	VUsMax            *stats.Metric
	Iterations        *stats.Metric
	IterationDuration *stats.Metric
	DroppedIterations *stats.Metric
	Errors            *stats.Metric

	// Runner-emitted.
	Checks        *stats.Metric
	GroupDuration *stats.Metric

	// HTTP-related.
	HTTPReqs              *stats.Metric
	HTTPReqFailed         *stats.Metric
	HTTPReqDuration       *stats.Metric
	HTTPReqBlocked        *stats.Metric
	HTTPReqConnecting     *stats.Metric
	HTTPReqTLSHandshaking *stats.Metric
	HTTPReqSending        *stats.Metric
	HTTPReqWaiting        *stats.Metric
	HTTPReqReceiving      *stats.Metric

	// Websocket-related
	WSSessions         *stats.Metric
	WSMessagesSent     *stats.Metric
	WSMessagesReceived *stats.Metric
	WSPing             *stats.Metric
	WSSessionDuration  *stats.Metric
	WSConnecting       *stats.Metric

	// gRPC-related
	GRPCReqDuration *stats.Metric

	// Network-related; used for future protocols as well.
	DataSent     *stats.Metric
	DataReceived *stats.Metric
}

func RegisterBuiltinMetrics(registry *stats.Registry) *BuiltInMetrics {
	return &BuiltInMetrics{
		VUs:               registry.MustNewMetric("vus", stats.Gauge),
		VUsMax:            registry.MustNewMetric("vus_max", stats.Gauge),
		Iterations:        registry.MustNewMetric("iterations", stats.Counter),
		IterationDuration: registry.MustNewMetric("iteration_duration", stats.Trend, stats.Time),
		DroppedIterations: registry.MustNewMetric("dropped_iterations", stats.Counter),
		Errors:            registry.MustNewMetric("errors", stats.Counter),

		Checks:        registry.MustNewMetric("checks", stats.Rate),
		GroupDuration: registry.MustNewMetric("group_duration", stats.Trend, stats.Time),

		HTTPReqs:              registry.MustNewMetric("http_reqs", stats.Counter),
		HTTPReqFailed:         registry.MustNewMetric("http_req_failed", stats.Rate),
		HTTPReqDuration:       registry.MustNewMetric("http_req_duration", stats.Trend, stats.Time),
		HTTPReqBlocked:        registry.MustNewMetric("http_req_blocked", stats.Trend, stats.Time),
		HTTPReqConnecting:     registry.MustNewMetric("http_req_connecting", stats.Trend, stats.Time),
		HTTPReqTLSHandshaking: registry.MustNewMetric("http_req_tls_handshaking", stats.Trend, stats.Time),
		HTTPReqSending:        registry.MustNewMetric("http_req_sending", stats.Trend, stats.Time),
		HTTPReqWaiting:        registry.MustNewMetric("http_req_waiting", stats.Trend, stats.Time),
		HTTPReqReceiving:      registry.MustNewMetric("http_req_receiving", stats.Trend, stats.Time),

		WSSessions:         registry.MustNewMetric("ws_sessions", stats.Counter),
		WSMessagesSent:     registry.MustNewMetric("ws_msgs_sent", stats.Counter),
		WSMessagesReceived: registry.MustNewMetric("ws_msgs_received", stats.Counter),
		WSPing:             registry.MustNewMetric("ws_ping", stats.Trend, stats.Time),
		WSSessionDuration:  registry.MustNewMetric("ws_session_duration", stats.Trend, stats.Time),
		WSConnecting:       registry.MustNewMetric("ws_connecting", stats.Trend, stats.Time),

		GRPCReqDuration: registry.MustNewMetric("grpc_req_duration", stats.Trend, stats.Time),

		DataSent:     registry.MustNewMetric("data_sent", stats.Counter, stats.Data),
		DataReceived: registry.MustNewMetric("data_received", stats.Counter, stats.Data),
	}
}

func WithBuiltinMetrics(ctx context.Context, b *BuiltInMetrics) context.Context {
	return context.WithValue(ctx, builtinMetricsKey, b)
}

func GetBuiltInMetrics(ctx context.Context) *BuiltInMetrics {
	if v := ctx.Value(builtinMetricsKey); v != nil {
		return v.(*BuiltInMetrics)
	}
	return nil
}

type contextKey uint64

const (
	builtinMetricsKey contextKey = iota
)
