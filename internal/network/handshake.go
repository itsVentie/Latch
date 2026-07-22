package network

import (
	"encoding/binary"
	"io"
)

type HandshakeHello struct {
	ClientEphPub  []byte
	ClientKEMPub  []byte
	SessionID     []byte
	SessionTicket []byte
}

type ServerHello struct {
	SessionResumed bool
	SessionID      []byte
	ServerEphPub   []byte
	ClientKEMCiph  []byte
	NewTicket      []byte
}

func WriteHandshakeHello(w io.Writer, h *HandshakeHello) error {
	idLen := len(h.SessionID)
	ephLen := len(h.ClientEphPub)
	kemLen := len(h.ClientKEMPub)

	header := make([]byte, 6)
	binary.BigEndian.PutUint16(header[0:2], uint16(idLen))
	binary.BigEndian.PutUint16(header[2:4], uint16(ephLen))
	binary.BigEndian.PutUint16(header[4:6], uint16(kemLen))

	if _, err := w.Write(header); err != nil {
		return err
	}
	if idLen > 0 {
		if _, err := w.Write(h.SessionID); err != nil {
			return err
		}
	}
	if ephLen > 0 {
		if _, err := w.Write(h.ClientEphPub); err != nil {
			return err
		}
	}
	if kemLen > 0 {
		if _, err := w.Write(h.ClientKEMPub); err != nil {
			return err
		}
	}
	return nil
}

func ReadHandshakeHello(r io.Reader) (*HandshakeHello, error) {
	header := make([]byte, 6)
	if _, err := io.ReadAtLeast(r, header, 6); err != nil {
		return nil, err
	}

	idLen := binary.BigEndian.Uint16(header[0:2])
	ephLen := binary.BigEndian.Uint16(header[2:4])
	kemLen := binary.BigEndian.Uint16(header[4:6])

	h := &HandshakeHello{
		SessionID:    make([]byte, idLen),
		ClientEphPub: make([]byte, ephLen),
		ClientKEMPub: make([]byte, kemLen),
	}

	if idLen > 0 {
		if _, err := io.ReadAtLeast(r, h.SessionID, int(idLen)); err != nil {
			return nil, err
		}
	}
	if ephLen > 0 {
		if _, err := io.ReadAtLeast(r, h.ClientEphPub, int(ephLen)); err != nil {
			return nil, err
		}
	}
	if kemLen > 0 {
		if _, err := io.ReadAtLeast(r, h.ClientKEMPub, int(kemLen)); err != nil {
			return nil, err
		}
	}
	return h, nil
}

func WriteServerHello(w io.Writer, s *ServerHello) error {
	resumedByte := byte(0)
	if s.SessionResumed {
		resumedByte = 1
	}

	idLen := len(s.SessionID)
	ephLen := len(s.ServerEphPub)
	ciphLen := len(s.ClientKEMCiph)

	header := make([]byte, 7)
	header[0] = resumedByte
	binary.BigEndian.PutUint16(header[1:3], uint16(idLen))
	binary.BigEndian.PutUint16(header[3:5], uint16(ephLen))
	binary.BigEndian.PutUint16(header[5:7], uint16(ciphLen))

	if _, err := w.Write(header); err != nil {
		return err
	}
	if idLen > 0 {
		if _, err := w.Write(s.SessionID); err != nil {
			return err
		}
	}
	if ephLen > 0 {
		if _, err := w.Write(s.ServerEphPub); err != nil {
			return err
		}
	}
	if ciphLen > 0 {
		if _, err := w.Write(s.ClientKEMCiph); err != nil {
			return err
		}
	}
	return nil
}

func ReadServerHello(r io.Reader) (*ServerHello, error) {
	header := make([]byte, 7)
	if _, err := io.ReadAtLeast(r, header, 7); err != nil {
		return nil, err
	}

	resumed := header[0] == 1
	idLen := binary.BigEndian.Uint16(header[1:3])
	ephLen := binary.BigEndian.Uint16(header[3:5])
	ciphLen := binary.BigEndian.Uint16(header[5:7])

	s := &ServerHello{
		SessionResumed: resumed,
		SessionID:      make([]byte, idLen),
		ServerEphPub:   make([]byte, ephLen),
		ClientKEMCiph:  make([]byte, ciphLen),
	}

	if idLen > 0 {
		if _, err := io.ReadAtLeast(r, s.SessionID, int(idLen)); err != nil {
			return nil, err
		}
	}
	if ephLen > 0 {
		if _, err := io.ReadAtLeast(r, s.ServerEphPub, int(ephLen)); err != nil {
			return nil, err
		}
	}
	if ciphLen > 0 {
		if _, err := io.ReadAtLeast(r, s.ClientKEMCiph, int(ciphLen)); err != nil {
			return nil, err
		}
	}
	return s, nil
}
