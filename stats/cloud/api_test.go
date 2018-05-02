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

package cloud

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func init() {
	_ = os.Setenv("K6CLOUD_HOST", "")
	_ = os.Setenv("K6CLOUD_TOKEN", "")
}

func TestCreateTestRun(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"reference_id": "1"}`)
	}))
	defer server.Close()

	client := NewClient("token", server.URL, "1.0")

	tr := &TestRun{
		Name: "test",
	}
	resp, err := client.CreateTestRun(tr)

	assert.Nil(t, err)
	assert.Equal(t, resp.ReferenceID, "1")
}

func TestPublishMetric(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "")
	}))
	defer server.Close()

	client := NewClient("token", server.URL, "1.0")

	samples := []*Sample{
		{
			Type:   "Point",
			Metric: "metric",
			Data: &SampleDataSingle{
				Type:  1,
				Time:  Timestamp(time.Now()),
				Value: 1.2,
			},
		},
	}
	err := client.PushMetric("1", false, samples)

	assert.Nil(t, err)
}

func TestFinished(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "")
	}))
	defer server.Close()

	client := NewClient("token", server.URL, "1.0")

	thresholds := map[string]map[string]bool{
		"threshold": {
			"max < 10": true,
		},
	}
	err := client.TestFinished("1", thresholds, true)

	assert.Nil(t, err)
}

func TestAuthorizedError(t *testing.T) {
	called := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, `{"error": {"code": 5, "message": "Not allowed"}}`)
	}))
	defer server.Close()

	client := NewClient("token", server.URL, "1.0")

	resp, err := client.CreateTestRun(&TestRun{Name: "test"})

	assert.Equal(t, 1, called)
	assert.Nil(t, resp)
	assert.EqualError(t, err, ErrNotAuthorized.Error())
}

func TestRetry(t *testing.T) {
	called := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(500)
	}))
	defer server.Close()

	client := NewClient("token", server.URL, "1.0")
	client.retryInterval = 1 * time.Millisecond
	resp, err := client.CreateTestRun(&TestRun{Name: "test"})

	assert.Equal(t, 3, called)
	assert.Nil(t, resp)
	assert.NotNil(t, err)
}

func TestRetrySuccessOnSecond(t *testing.T) {
	called := 1
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		if called == 2 {
			fmt.Fprintf(w, `{"reference_id": "1"}`)
			return
		}
		w.WriteHeader(500)
	}))
	defer server.Close()

	client := NewClient("token", server.URL, "1.0")
	client.retryInterval = 1 * time.Millisecond
	resp, err := client.CreateTestRun(&TestRun{Name: "test"})

	assert.Equal(t, 2, called)
	assert.NotNil(t, resp)
	assert.Nil(t, err)
}
