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

package cloudapi

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"testing"
	"time"

	gomock "github.com/golang/mock/gomock"
	"github.com/mailru/easyjson"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/lib/testutils"
)

func TestMsgParsing(t *testing.T) {
	m := `{
  "streams": [
    {
      "stream": {
      	"key1": "value1",
	   	"key2": "value2"
      },
      "values": [
        [
      	"1598282752000000000",
		"something to log"
        ]
      ]
    }
  ],
  "dropped_entries": [
    {
      "labels": {
      	"key3": "value1",
	   	"key4": "value2"
      },
      "timestamp": "1598282752000000000"
    }
  ]
}
`
	expectMsg := msg{
		Streams: []msgStreams{
			{
				Stream: map[string]string{"key1": "value1", "key2": "value2"},
				Values: [][2]string{{"1598282752000000000", "something to log"}},
			},
		},
		DroppedEntries: []msgDroppedEntries{
			{
				Labels:    map[string]string{"key3": "value1", "key4": "value2"},
				Timestamp: "1598282752000000000",
			},
		},
	}
	var message msg
	require.NoError(t, easyjson.Unmarshal([]byte(m), &message))
	require.Equal(t, expectMsg, message)
}

func TestMSGLog(t *testing.T) {
	expectMsg := msg{
		Streams: []msgStreams{
			{
				Stream: map[string]string{"key1": "value1", "key2": "value2"},
				Values: [][2]string{{"1598282752000000000", "something to log"}},
			},
			{
				Stream: map[string]string{"key1": "value1", "key2": "value2", "level": "warn"},
				Values: [][2]string{{"1598282752000000000", "something else log"}},
			},
		},
		DroppedEntries: []msgDroppedEntries{
			{
				Labels:    map[string]string{"key3": "value1", "key4": "value2", "level": "panic"},
				Timestamp: "1598282752000000000",
			},
		},
	}

	logger := logrus.New()
	logger.Out = ioutil.Discard
	hook := &testutils.SimpleLogrusHook{HookedLevels: logrus.AllLevels}
	logger.AddHook(hook)
	expectMsg.Log(logger)
	logLines := hook.Drain()
	assert.Equal(t, 4, len(logLines))
	expectTime := time.Unix(0, 1598282752000000000)
	for i, entry := range logLines {
		var expectedMsg string
		switch i {
		case 0:
			expectedMsg = "something to log"
		case 1:
			expectedMsg = "last message had unknown level "
		case 2:
			expectedMsg = "something else log"
		case 3:
			expectedMsg = "dropped"
		}
		require.Equal(t, expectedMsg, entry.Message)
		require.Equal(t, expectTime, entry.Time)
	}
}

func TestWSConnReadMessage(t *testing.T) {
	t.Parallel()

	wsurl := "ws://testurl"
	wsh := http.Header{}
	wsh.Add("key1", "v1")
	expmsg := []byte("Hello world!")

	t.Run("SuccessNoRequiredReconnect", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		dialer := NewMockLogtailStreamDialer(ctrl)
		conn := NewMockLogtailStreamer(ctrl)

		conn.EXPECT().ReadMessage().Return(0, expmsg, nil)

		wsc := wsconn{
			m:          sync.Mutex{},
			conn:       conn,
			dialer:     dialer,
			dialConfig: dialConfig{url: wsurl, headers: wsh},
		}
		m, err := wsc.ReadMessage(context.Background())
		require.NoError(t, err)
		assert.Equal(t, expmsg, m)
	})
	t.Run("SuccessRequiredReconnect", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		dialer := NewMockLogtailStreamDialer(ctrl)
		conn := NewMockLogtailStreamer(ctrl)

		dialer.EXPECT().DialContext(context.Background(), wsurl, wsh).Return(conn, nil)
		firstCall := conn.EXPECT().ReadMessage().Return(0, nil, fmt.Errorf("unexpected error"))
		conn.EXPECT().ReadMessage().After(firstCall).Return(0, expmsg, nil)
		conn.EXPECT().WriteControl(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		conn.EXPECT().Close().Return(nil)

		var redialCount int
		wsc := wsconn{
			m:          sync.Mutex{},
			conn:       conn,
			dialer:     dialer,
			dialConfig: dialConfig{url: wsurl, headers: wsh},
			onRedialing: func() {
				redialCount++
			},
		}

		m, err := wsc.ReadMessage(context.Background())
		require.NoError(t, err)

		assert.Equal(t, expmsg, m)
		assert.Equal(t, 1, redialCount)
	})
	t.Run("Fail", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		dialer := NewMockLogtailStreamDialer(ctrl)
		conn := NewMockLogtailStreamer(ctrl)

		dialer.EXPECT().DialContext(context.Background(), wsurl, wsh).Return(conn, nil)
		conn.EXPECT().Close().Return(nil)
		firstCall := conn.EXPECT().ReadMessage().Return(0, nil, fmt.Errorf("unexpected error"))
		conn.EXPECT().ReadMessage().After(firstCall).Return(0, nil, fmt.Errorf("a second error"))
		conn.EXPECT().WriteControl(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

		wsc := wsconn{
			m:           sync.Mutex{},
			conn:        conn,
			dialer:      dialer,
			dialConfig:  dialConfig{url: wsurl, headers: wsh},
			onRedialing: func() {},
		}
		m, err := wsc.ReadMessage(context.Background())
		assert.Error(t, err, "a second error")
		assert.Nil(t, m)
	})
}

func TestWSConnKeepAlive(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	dialer := NewMockLogtailStreamDialer(ctrl)
	conn := NewMockLogtailStreamer(ctrl)

	wsurl := "ws://testurl"
	wsh := http.Header{}
	wsh.Add("key1", "v1")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wg := sync.WaitGroup{}

	// it asserts that the dial is invoked when the timer fires.
	// and that the retry logic is invoked in case of failure
	firstCall := dialer.EXPECT().DialContext(ctx, wsurl, wsh).Return(nil, fmt.Errorf("unexpected error"))
	dialer.EXPECT().DialContext(ctx, wsurl, wsh).After(firstCall).DoAndReturn(func(_ context.Context, _ string, _ http.Header) (LogtailStreamer, error) {
		// stop the keep-alive listener
		cancel()
		wg.Done()
		// the stream conn is not required for this test case
		return nil, nil
	})
	conn.EXPECT().WriteControl(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	conn.EXPECT().Close().Return(nil)

	clockch := make(chan time.Time)
	defer close(clockch)

	var redialCount int
	wsc := wsconn{
		m:      sync.Mutex{},
		conn:   conn,
		dialer: dialer,
		dialConfig: dialConfig{
			url:           wsurl,
			headers:       wsh,
			retryAttempts: 2,
		},
		afterfn: func(d time.Duration) <-chan time.Time {
			return clockch
		},
		onRedialing: func() {
			redialCount++
		},
	}

	wg.Add(1)
	go wsc.keepAlive(ctx, time.Second)
	clockch <- time.Now()
	wg.Wait()

	assert.Equal(t, 1, redialCount)
}
