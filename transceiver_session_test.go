package gosmpp

import (
	"bytes"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/linxGnu/gosmpp/pdu"
	"github.com/stretchr/testify/require"
)

func TestRebind(t *testing.T) {
	t.Run("RebindWhenWriteTimeout", func(t *testing.T) {
		// setup mock connection
		conn := &mockConnection{
			lock:     &sync.Mutex{},
			toBeRead: &bytes.Buffer{},
			written:  &bytes.Buffer{},
			add:      &mockAddr{},
			closed:   0,
		}
		// set toBeRead with valid BindResp pdu data
		pdu.NewBindTransceiverResp().Marshal(&pdu.ByteBuffer{conn.toBeRead})

		// create new session
		var (
			hasSubmitError int32
			hasRebined     int32
		)

		setting := TransceiveSettings{
			WriteTimeout: 300 * time.Millisecond,
			EnquireLink:  300 * time.Millisecond,
			OnSubmitError: func(pdu pdu.PDU, err error) {
				if err != nil {
					atomic.CompareAndSwapInt32(&hasSubmitError, 0, 1)
				}
			},
			OnReceivingError: nil,
			OnRebindingError: nil,
			OnPDU:            nil,
			OnClosed: func(state State) {
				if state != ExplicitClosing {
					t.Logf("rebind with state %v", state)
					atomic.CompareAndSwapInt32(&hasRebined, 0, 1)
				}
			},
		}

		session, err := NewTransceiverSession(
			func(string) (net.Conn, error) {
				return conn, nil
			},
			Auth{"test", "test", "test", "test"},
			setting,
			time.Second)
		require.NoError(t, err)

		// set connection.err to timeout
		conn.SetErr(&mockNetErr{
			error:   fmt.Errorf("connection has timeout"),
			timeout: 1,
			temp:    0,
		})

		session.Transceiver().Submit(pdu.NewSubmitSM())

		// setup OnClose() with require.Condition to wait for conn timeout
		require.Eventually(t, func() bool {
			return atomic.LoadInt32(&hasRebined) == 1
		}, 10*time.Second, 300*time.Millisecond,
			"trancevier not rebinding after 10 second of connection write timeout")

		// setup OnClose() with require.Condition to wait for conn timeout
		require.Eventually(t, func() bool {
			return atomic.LoadInt32(&hasSubmitError) == 1
		}, 10*time.Second, 300*time.Millisecond,
			"trancevier should have on submit error indicates timeout")
	})

	t.Run("RebindWhenReadEOF", func(t *testing.T) {
		// setup mock connection
		conn := &mockConnection{
			lock:     &sync.Mutex{},
			toBeRead: &bytes.Buffer{},
			written:  &bytes.Buffer{},
			add:      &mockAddr{},
			closed:   0,
		}
		// set toBeRead with valid BindResp pdu data
		pdu.NewBindTransceiverResp().Marshal(&pdu.ByteBuffer{conn.toBeRead})

		// create new session
		var (
			hasReceivedError int32
			hasRebined       int32
		)

		setting := TransceiveSettings{
			WriteTimeout: 300 * time.Millisecond,
			EnquireLink:  300 * time.Millisecond,
			OnReceivingError: func(err error) {
				if err != nil {
					t.Log(err.Error())
					atomic.CompareAndSwapInt32(&hasReceivedError, 0, 1)
				}
			},
			OnClosed: func(state State) {
				if state != ExplicitClosing {
					t.Logf("rebind with state %v", state)
					atomic.CompareAndSwapInt32(&hasRebined, 0, 1)
				}
			},
		}

		_, err := NewTransceiverSession(
			func(string) (net.Conn, error) {
				return conn, nil
			},
			Auth{"test", "test", "test", "test"},
			setting,
			time.Second)
		require.NoError(t, err)

		conn.toBeRead.Write([]byte("ABD512BACA33"))

		// setup OnClose() with require.Condition to wait for conn timeout
		require.Eventually(t, func() bool {
			return atomic.LoadInt32(&hasRebined) == 1
		}, 10*time.Second, 300*time.Millisecond,
			"trancevier not rebinding after 10 second of connection read EOF")

		// setup OnClose() with require.Condition to wait for conn timeout
		require.Eventually(t, func() bool {
			return atomic.LoadInt32(&hasReceivedError) == 1
		}, 10*time.Second, 300*time.Millisecond,
			"trancevier should have hasReceivedError indicates EOF")
	})
}

func RebindWaitAtLeast(t *testing.T) {
	// setup mock connection
	conn := &mockConnection{
		lock:     &sync.Mutex{},
		toBeRead: &bytes.Buffer{},
		written:  &bytes.Buffer{},
		add:      &mockAddr{},
		closed:   0,
	}
	// set toBeRead with valid BindResp pdu data
	pdu.NewBindTransceiverResp().Marshal(&pdu.ByteBuffer{conn.toBeRead})

	// create new session
	var (
		rebindAttemptsCounter int32
		// totalRebindInterval sleep on rebind error + rebindIntervalMS
		totalRebindInterval = 2*time.Second + 512*time.Millisecond
	)

	setting := TransceiveSettings{
		WriteTimeout: 300 * time.Millisecond,
		EnquireLink:  300 * time.Millisecond,
		OnRebindingError: func(err error) {
			time.After(time.Second + 512*time.Millisecond)
		},
		OnClosed: func(state State) {
			if state != ExplicitClosing {
				t.Logf("rebind with state %v", state)
				atomic.AddInt32(&rebindAttemptsCounter, 1)
			}
		},
	}

	_, err := NewTransceiverSession(
		func(string) (net.Conn, error) {
			return conn, nil
		},
		Auth{"test", "test", "test", "test"},
		setting,
		time.Second)
	require.NoError(t, err)

	// set connection.err to timeout
	conn.toBeRead.Write([]byte("ABD512BACA33"))

	// check that the second rebind attempt must not be less than 2 and half second later
	require.Never(t, func() bool {
		return atomic.LoadInt32(&rebindAttemptsCounter) < 2
	}, totalRebindInterval-12*time.Millisecond, 100*time.Millisecond)
}
