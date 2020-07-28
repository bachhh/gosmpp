package gosmpp

import (
	"bytes"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// mockConnection stub implements net.Conn interface for testing
type mockConnection struct {
	lock     *sync.Mutex
	toBeRead *bytes.Buffer // toBeRead store data for Read()
	written  *bytes.Buffer // written store data passed into Write()
	err      net.Error     // error

	add    *mockAddr
	closed int32 // int32
}

type mockAddr struct{}

func (a *mockAddr) Network() string {
	return "mock"
}
func (a *mockAddr) String() string {
	return "127.0.0.1:10800"
}

// SetErr set custom error
func (m *mockConnection) SetErr(err net.Error) {
	m.lock.Lock()
	m.err = err
	m.lock.Unlock()
}

// SetReadByte set custom data for subsequent Read(), clears all previous data
func (m *mockConnection) SetReadData(rd []byte) {
	m.lock.Lock()
	m.toBeRead = bytes.NewBuffer(rd)
	m.lock.Unlock()
}

// AddReadDAta add more custom data for subsequent Read()
func (m *mockConnection) AddReadData(rd []byte) (err error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	_, err = m.toBeRead.Write(rd)
	return
}

func (m *mockConnection) WrittenData() (rd []byte) {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.written.Bytes()
}

// standard net.conn
func (m *mockConnection) Read(b []byte) (n int, err error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.toBeRead.Read(b)
}

func (m *mockConnection) Write(b []byte) (n int, err error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.written.Write(b)
}

func (m *mockConnection) Close() error {
	atomic.CompareAndSwapInt32(&m.closed, 0, 1)
	return m.err
}

func (m *mockConnection) LocalAddr() net.Addr {
	return m.add
}
func (m *mockConnection) RemoteAddr() net.Addr {
	return m.add
}
func (m *mockConnection) SetDeadline(t time.Time) error {
	return nil
}
func (m *mockConnection) SetReadDeadline(t time.Time) error {
	return nil
}
func (m *mockConnection) SetWriteDeadline(t time.Time) error {
	return nil
}

type mockNetErr struct {
	error
	timeout int32
	temp    int32
}

func (m *mockNetErr) SetTimeout() {
	atomic.StoreInt32(&m.timeout, 1)
}

func (m *mockNetErr) SetTemporary() {
	atomic.StoreInt32(&m.temp, 1)
}

func (m *mockNetErr) Timeout() bool {
	return atomic.LoadInt32(&m.timeout) == 1
}
func (m *mockNetErr) Temporary() bool {
	return atomic.LoadInt32(&m.temp) == 1
}
