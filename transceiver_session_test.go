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
			hasReceivedError int32
			hasRebined       int32
		)

		setting := TransceiveSettings{
			WriteTimeout: 300 * time.Millisecond,
			EnquireLink:  300 * time.Millisecond,
			OnSubmitError: func(pdu pdu.PDU, err error) {
				if err != nil {
					atomic.CompareAndSwapInt32(&hasReceivedError, 0, 1)
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

		require.Error(t, session.Transceiver().Submit(pdu.NewSubmitSM()))

		// setup OnClose() with require.Condition to wait for conn timeout
		require.Eventually(t, func() bool {
			return atomic.LoadInt32(&hasRebined) == 1
		}, 10*time.Second, 300*time.Millisecond,
			"trancevier not rebinding after 10 second of connection write timeout")

		// setup OnClose() with require.Condition to wait for conn timeout
		require.Eventually(t, func() bool {
			return atomic.LoadInt32(&hasReceivedError) == 1
		}, 10*time.Second, 300*time.Millisecond,
			"trancevier should have on submit error indicates timeout")

	})

	t.Run("RebindWhenReadEOF", func(t *testing.T) {
	})
}
