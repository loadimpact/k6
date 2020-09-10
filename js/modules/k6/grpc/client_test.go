/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2020 Load Impact
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

package grpc

import (
	"context"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/grpc_testing"
	"gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/testutils/httpmultibin"
	"github.com/loadimpact/k6/stats"
)

func TestClient(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)
	defer tb.Cleanup()
	sr := tb.Replacer.Replace

	root, err := lib.NewGroup("", nil)
	assert.NoError(t, err)

	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})
	samples := make(chan stats.SampleContainer, 1000)
	state := &lib.State{
		Group:     root,
		Dialer:    tb.Dialer,
		TLSConfig: tb.TLSClientConfig,
		Samples:   samples,
		Options: lib.Options{
			SystemTags: stats.NewSystemTagSet(
				stats.TagName,
			),
			UserAgent: null.StringFrom("k6-test"),
		},
	}

	ctx := common.WithRuntime(context.Background(), rt)

	rt.Set("grpc", common.Bind(rt, New(), &ctx))

	t.Run("New", func(t *testing.T) {
		_, err := common.RunString(rt, `
			var client = grpc.newClient();
			if (!client) throw new Error("no client created")
		`)
		assert.NoError(t, err)
	})

	t.Run("LoadNotFound", func(t *testing.T) {
		_, err := common.RunString(rt, `
			client.load([], "./does_not_exist.proto");	
		`)
		if !assert.Error(t, err) {
			return
		}
		assert.Contains(t, err.Error(), "no such file or directory")
	})

	t.Run("Load", func(t *testing.T) {
		respV, err := common.RunString(rt, `
			client.load([], "../../../../vendor/google.golang.org/grpc/test/grpc_testing/test.proto");	
		`)
		if !assert.NoError(t, err) {
			return
		}
		resp := respV.Export()
		assert.IsType(t, []MethodDesc{}, resp)
		assert.Len(t, resp, 6)
	})

	t.Run("ConnectInit", func(t *testing.T) {
		_, err := common.RunString(rt, `
			var err = client.connect();
			throw new Error(err)
		`)
		if !assert.Error(t, err) {
			return
		}
		assert.Contains(t, err.Error(), "connecting to a gRPC server in the init context is not supported")
	})

	t.Run("invokeRPCInit", func(t *testing.T) {
		_, err := common.RunString(rt, `
			var err = client.invokeRPC();
			throw new Error(err)
		`)
		if !assert.Error(t, err) {
			return
		}
		assert.Contains(t, err.Error(), "invoking RPC methods in the init context is not supported")
	})

	ctx = lib.WithState(ctx, state)

	t.Run("NoConnect", func(t *testing.T) {
		_, err := common.RunString(rt, `
			client.invokeRPC("grpc.testing.TestService/EmptyCall", {})
		`)
		if !assert.Error(t, err) {
			return
		}
		assert.Contains(t, err.Error(), "no gRPC connection, you must call connect first")
	})

	t.Run("Connect", func(t *testing.T) {
		_, err := common.RunString(rt, sr(`
			var err = client.connect("GRPCBIN_ADDR"); 
			if (err) throw new Error("connection failed with error: " + err)
		`))
		assert.NoError(t, err)
	})

	t.Run("InvokeRPCNotFound", func(t *testing.T) {
		_, err := common.RunString(rt, `
			client.invokeRPC("foo/bar", {})
		`)
		if !assert.Error(t, err) {
			return
		}
		assert.Contains(t, err.Error(), "method \"foo/bar\" not found in file descriptors")
	})

	t.Run("InvokeRPC", func(t *testing.T) {
		tb.GRPCStub.EmptyCallFunc = func(context.Context, *grpc_testing.Empty) (*grpc_testing.Empty, error) {
			return &grpc_testing.Empty{}, nil
		}
		_, err := common.RunString(rt, `
			var resp = client.invokeRPC("grpc.testing.TestService/EmptyCall", {})
			if (resp.status !== grpc.StatusOK) {
				throw new Error("unexpected error status: " + resp.status)
			}
		`)
		assert.NoError(t, err)
	})

	t.Run("RequestMessage", func(t *testing.T) {
		tb.GRPCStub.UnaryCallFunc = func(_ context.Context, req *grpc_testing.SimpleRequest) (*grpc_testing.SimpleResponse, error) {
			if req.Payload == nil || string(req.Payload.Body) != "负载测试" {
				return nil, status.Error(codes.InvalidArgument, "")
			}
			return &grpc_testing.SimpleResponse{}, nil
		}
		_, err := common.RunString(rt, `
			var resp = client.invokeRPC("grpc.testing.TestService/UnaryCall", { payload: { body: "6LSf6L295rWL6K+V"} })
			if (resp.status !== grpc.StatusOK) {
				throw new Error("server did not receive the correct request message")
			}
		`)
		assert.NoError(t, err)
	})

	t.Run("RequestHeaders", func(t *testing.T) {
		tb.GRPCStub.EmptyCallFunc = func(ctx context.Context, _ *grpc_testing.Empty) (*grpc_testing.Empty, error) {
			md, ok := metadata.FromIncomingContext(ctx)
			if !ok || len(md["x-load-tester"]) == 0 || md["x-load-tester"][0] != "k6" {
				return nil, status.Error(codes.FailedPrecondition, "")
			}

			return &grpc_testing.Empty{}, nil
		}
		_, err := common.RunString(rt, `
			var resp = client.invokeRPC("grpc.testing.TestService/EmptyCall", {}, { headers: { "X-Load-Tester": "k6" } })
			if (resp.status !== grpc.StatusOK) {
				throw new Error("failed to send correct headers in the request")
			}
		`)
		assert.NoError(t, err)
	})

	t.Run("ResponseMessage", func(t *testing.T) {
		tb.GRPCStub.UnaryCallFunc = func(context.Context, *grpc_testing.SimpleRequest) (*grpc_testing.SimpleResponse, error) {
			return &grpc_testing.SimpleResponse{
				Username:   "k6",
				OauthScope: "水",
			}, nil
		}
		_, err := common.RunString(rt, `
			var resp = client.invokeRPC("grpc.testing.TestService/UnaryCall", {})
			if (!resp.message || resp.message.username !== "k6" || resp.message.oauthScope !== "水") {
				throw new Error("unexpected response message: " + JSON.stringify(resp.message))
			}
		`)
		assert.NoError(t, err)
	})

	t.Run("ResponseError", func(t *testing.T) {
		tb.GRPCStub.EmptyCallFunc = func(context.Context, *grpc_testing.Empty) (*grpc_testing.Empty, error) {
			return nil, status.Error(codes.DataLoss, "foobar")
		}
		_, err := common.RunString(rt, `
			var resp = client.invokeRPC("grpc.testing.TestService/EmptyCall", {})
			if (resp.status !== grpc.StatusDataLoss) {
				throw new Error("unexpected error status: " + resp.status)
			}
			if (!resp.error || resp.error.message !== "foobar" || resp.error.code !== 15) {
				throw new Error("unexpected error object: " + JSON.stringify(resp.error))
			}
		`)
		assert.NoError(t, err)
	})

	t.Run("ResponseHeaders", func(t *testing.T) {
		tb.GRPCStub.EmptyCallFunc = func(ctx context.Context, _ *grpc_testing.Empty) (*grpc_testing.Empty, error) {
			md := metadata.Pairs("foo", "bar")
			grpc.SetHeader(ctx, md)
			return &grpc_testing.Empty{}, nil
		}
		_, err := common.RunString(rt, `
			var resp = client.invokeRPC("grpc.testing.TestService/EmptyCall", {})
			if (resp.status !== grpc.StatusOK) {
				throw new Error("unexpected error status: " + resp.status)
			}
			if (!resp.headers || !resp.headers["foo"] || resp.headers["foo"][0] !== "bar") {
				throw new Error("unexpected headers object: " + JSON.stringify(resp.trailers))
			}
		`)
		assert.NoError(t, err)
	})

	t.Run("ResponseTrailers", func(t *testing.T) {
		tb.GRPCStub.EmptyCallFunc = func(ctx context.Context, _ *grpc_testing.Empty) (*grpc_testing.Empty, error) {
			md := metadata.Pairs("foo", "bar")
			grpc.SetTrailer(ctx, md)
			return &grpc_testing.Empty{}, nil
		}
		_, err := common.RunString(rt, `
			var resp = client.invokeRPC("grpc.testing.TestService/EmptyCall", {})
			if (resp.status !== grpc.StatusOK) {
				throw new Error("unexpected error status: " + resp.status)
			}
			if (!resp.trailers || !resp.trailers["foo"] || resp.trailers["foo"][0] !== "bar") {
				throw new Error("unexpected trailers object: " + JSON.stringify(resp.trailers))
			}
		`)
		assert.NoError(t, err)
	})

	t.Run("LoadNotInit", func(t *testing.T) {
		_, err := common.RunString(rt, "client.load()")
		if !assert.Error(t, err) {
			return
		}
		assert.Contains(t, err.Error(), "load must be called in the init context")
	})

	t.Run("Close", func(t *testing.T) {
		_, err := common.RunString(rt, `
			client.close();
			client.invokeRPC();
		`)
		if !assert.Error(t, err) {
			return
		}
		assert.Contains(t, err.Error(), "no gRPC connection")
	})
}
