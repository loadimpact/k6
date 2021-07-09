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
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mailru/easyjson"
	"github.com/sirupsen/logrus"
)

//go:generate easyjson -pkg -no_std_marshalers -gen_build_flags -mod=mod .

//easyjson:json
type msg struct {
	Streams        []msgStreams        `json:"streams"`
	DroppedEntries []msgDroppedEntries `json:"dropped_entries"`
}

//easyjson:json
type msgStreams struct {
	Stream map[string]string `json:"stream"`
	Values [][2]string       `json:"values"` // this can be optimized
}

//easyjson:json
type msgDroppedEntries struct {
	Labels    map[string]string `json:"labels"`
	Timestamp string            `json:"timestamp"`
}

func (m *msg) Log(logger logrus.FieldLogger) {
	var level string

	for _, stream := range m.Streams {
		fields := labelsToLogrusFields(stream.Stream)
		var ok bool
		if level, ok = stream.Stream["level"]; ok {
			delete(fields, "level")
		}

		for _, value := range stream.Values {
			nsec, _ := strconv.Atoi(value[0])
			e := logger.WithFields(fields).WithTime(time.Unix(0, int64(nsec)))
			lvl, err := logrus.ParseLevel(level)
			if err != nil {
				e.Info(value[1])
				e.Warn("last message had unknown level " + level)
			} else {
				e.Log(lvl, value[1])
			}
		}
	}

	for _, dropped := range m.DroppedEntries {
		nsec, _ := strconv.Atoi(dropped.Timestamp)
		logger.WithFields(labelsToLogrusFields(dropped.Labels)).WithTime(time.Unix(0, int64(nsec))).Warn("dropped")
	}
}

func labelsToLogrusFields(labels map[string]string) logrus.Fields {
	fields := make(logrus.Fields, len(labels))

	for key, val := range labels {
		fields[key] = val
	}

	return fields
}

func (c *Config) getRequest(referenceID string, start time.Duration) (*url.URL, error) {
	u, err := url.Parse(c.LogsTailURL.String)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse cloud logs host %w", err)
	}

	u.RawQuery = fmt.Sprintf(`query={test_run_id="%s"}&start=%d`,
		referenceID,
		time.Now().Add(-start).UnixNano(),
	)

	return u, nil
}

// StreamLogsToLogger streams the logs for the configured test to the provided logger until ctx is
// Done or an error occurs.
func (c *Config) StreamLogsToLogger(
	ctx context.Context, logger logrus.FieldLogger, referenceID string, start time.Duration,
) error {
	u, err := c.getRequest(referenceID, start)
	if err != nil {
		return err
	}

	headers := make(http.Header)
	headers.Add("Sec-WebSocket-Protocol", "token="+c.Token.String)

	conn, err := dialContext(ctx, dialConfig{
		url:           u.String(),
		headers:       headers,
		retryAttempts: c.LogsTailDialRetryAttempts.Int64,
		retryInterval: time.Duration(c.LogsTailDialRetryInterval.Duration),
		refreshAfter:  time.Duration(c.LogsTailRefreshAfter.Duration),
	})
	if err != nil {
		return err
	}
	conn.OnRedialing(func() {
		logger.Info("we are reconnecting to tail logs, this might result in either some repeated or missed messages")
	})

	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	msgBuffer := make(chan []byte, 10)
	defer close(msgBuffer)

	go func() {
		for message := range msgBuffer {
			var m msg
			err := easyjson.Unmarshal(message, &m)
			if err != nil {
				logger.WithError(err).Errorf("couldn't unmarshal a message from the cloud: %s", string(message))

				continue
			}
			m.Log(logger)
		}
	}()

	for {
		message, err := conn.ReadMessage(ctx)
		select { // check if we should stop before continuing
		case <-ctx.Done():
			return nil
		default:
		}

		if err != nil {
			logger.WithError(err).Warn("error reading a message from the cloud")
			return err
		}

		select {
		case <-ctx.Done():
			return nil
		case msgBuffer <- message:
		}
	}
}

//nolint:lll
//go:generate mockgen -package cloudapi -destination ./logtail_streamer_mock.go --build_flags=--mod=mod . LogtailStreamer

// LogtailStreamer represents a streaming connection with a logtail service.
type LogtailStreamer interface {
	ReadMessage() (msgtype int, msg []byte, err error)
	WriteControl(msgtype int, msg []byte, deadline time.Time) error
	Close() error
}

//nolint:lll
//go:generate mockgen -package cloudapi -destination ./logtail_dialer_mock.go --build_flags=--mod=mod . LogtailStreamDialer

// LogtailStreamDialer is a streaming connection dialer with a logtail service.
type LogtailStreamDialer interface {
	DialContext(ctx context.Context, url string, headers http.Header) (LogtailStreamer, error)
}

// dialAdapter adapts a gorilla websocket dialer as a LogtailStreamDialer.
type dialAdapter struct {
	dialer *websocket.Dialer
}

func (a dialAdapter) DialContext(ctx context.Context, url string, headers http.Header) (LogtailStreamer, error) {
	// We don't need to close the http body or use it for anything until we want to actually log
	// what the server returned as body when it errors out
	conn, _, err := a.dialer.DialContext(ctx, url, headers) //nolint:bodyclose
	return conn, err
}

// dialContext returns an established websocket connection with the logtail stream service.
func dialContext(ctx context.Context, c dialConfig) (*wsconn, error) {
	wsc := &wsconn{
		m:          sync.Mutex{},
		conn:       nil,
		dialer:     dialAdapter{dialer: websocket.DefaultDialer},
		dialConfig: c,
		afterfn:    time.After,
		onRedialing: func() {
		},
	}
	if err := wsc.dial(ctx); err != nil {
		return nil, err
	}
	if wsc.dialConfig.refreshAfter > 0 {
		go wsc.keepAlive(ctx, wsc.dialConfig.refreshAfter)
	}
	return wsc, nil
}

type wsconn struct {
	m          sync.Mutex
	conn       LogtailStreamer
	dialer     LogtailStreamDialer
	dialConfig dialConfig

	// afterfn abstracts a timer
	afterfn func(time.Duration) <-chan time.Time

	// onRedialing is a callback invoked
	// when a connection's refresh attempt is invoked.
	onRedialing func()
}

type dialConfig struct {
	url           string
	headers       http.Header
	retryAttempts int64
	retryInterval time.Duration
	refreshAfter  time.Duration
}

// OnRedialing sets a function to invoke
// when the a new connection refresh is on going.
func (wsc *wsconn) OnRedialing(f func()) {
	wsc.onRedialing = f
}

func (wsc *wsconn) dial(ctx context.Context) error {
	attempts := uint(1)
	if wsc.dialConfig.retryAttempts > 0 {
		attempts = uint(wsc.dialConfig.retryAttempts)
	}
	var newc LogtailStreamer
	err := retry(attempts, wsc.dialConfig.retryInterval, func() (err error) {
		select {
		case <-ctx.Done():
			return
		default:
			newc, err = wsc.dialer.DialContext(ctx, wsc.dialConfig.url, wsc.dialConfig.headers)
		}
		return
	})
	if err != nil {
		return err
	}

	// if any, close the previous conn
	if wsc.conn != nil {
		_ = wsc.Close()
	}
	wsc.m.Lock()
	wsc.conn = newc
	wsc.m.Unlock()
	return nil
}

// keepAlive keeps alive the websocket connection invoking a new dial
// every interval defined from refreshAfter.
func (wsc *wsconn) keepAlive(ctx context.Context, refreshAfter time.Duration) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-wsc.afterfn(refreshAfter):
			wsc.onRedialing()
			// error skipped to let it fail on reading
			// or retry a new dial on the next timer firing
			_ = wsc.dial(ctx)
		}
	}
}

// ReadMessge reads a message from the stream.
func (wsc *wsconn) ReadMessage(ctx context.Context) ([]byte, error) {
	readfn := func() (messageType int, p []byte, err error) {
		wsc.m.Lock()
		messageType, p, err = wsc.conn.ReadMessage()
		wsc.m.Unlock()
		return
	}

	isErr := func(err error) bool {
		eclose := &websocket.CloseError{}
		return err != nil && !errors.As(err, &eclose)
	}

	_, msg, err := readfn()
	if isErr(err) {
		// try to fix getting a fresh connection
		wsc.onRedialing()
		dErr := wsc.dial(ctx)
		if dErr != nil {
			// return the main error
			return nil, err
		}
		_, msg, err = readfn()
		if isErr(err) {
			return nil, err
		}
	}
	return msg, nil
}

// Close closes the connection.
func (wsc *wsconn) Close() error {
	_ = wsc.conn.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseGoingAway, "closing"),
		time.Now().Add(time.Second))

	wsc.m.Lock()
	_ = wsc.conn.Close()
	wsc.m.Unlock()
	return nil
}

// retry retries a to exeucute a provided function until it isn't succeeded
// or maximum number of attempts is hit. It waits the specificed interval
// between the latest iteration and the next retry.
func retry(attempts uint, interval time.Duration, do func() error) (err error) {
	for i := 0; i < int(attempts); i++ {
		if i > 0 {
			time.Sleep(interval)
		}
		err = do()
		if err == nil {
			return nil
		}
	}
	return
}
