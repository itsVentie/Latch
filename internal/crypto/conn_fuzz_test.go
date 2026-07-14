package crypto

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"
)

type mockAddr struct{}

func (mockAddr) Network() string { return "tcp" }
func (mockAddr) String() string  { return "127.0.0.1:1234" }

type dummyConn struct {
	io.Reader
	io.Writer
}

func (d dummyConn) Close() error                       { return nil }
func (d dummyConn) LocalAddr() net.Addr                { return mockAddr{} }
func (d dummyConn) RemoteAddr() net.Addr               { return mockAddr{} }
func (d dummyConn) SetDeadline(t time.Time) error      { return nil }
func (d dummyConn) SetReadDeadline(t time.Time) error  { return nil }
func (d dummyConn) SetWriteDeadline(t time.Time) error { return nil }

func FuzzSecureConnRead(f *testing.F) {
	f.Add([]byte{0x00, 0x05, 0x01, 0x02, 0x03, 0x04, 0x05})
	f.Add([]byte{0xff, 0xff})
	f.Add([]byte{0x00, 0x00})
	f.Add([]byte{0x00, 0x01, 0x02})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}

		masterKey := make([]byte, 32)
		for i := range masterKey {
			masterKey[i] = 0x42
		}

		mockInput := bytes.NewReader(data)
		var mockOutput bytes.Buffer
		conn := dummyConn{
			Reader: mockInput,
			Writer: &mockOutput,
		}

		sc, err := NewSecureConn(conn, masterKey)
		if err != nil {
			return
		}

		SetServerRoles(sc)

		outBuf := make([]byte, MaxFrameSize*2)
		for {
			_, err := sc.Read(outBuf)
			if err != nil {
				break
			}
		}
	})
}
