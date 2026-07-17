package network

import (
	"encoding/hex"
	"encoding/json"
	"net"
)

type SessionContext struct {
	SessionID  string `json:"session_id"`
	CryptoKey  string `json:"crypto_key"`
	ListenAddr string `json:"listen_addr"`
	RemoteAddr string `json:"remote_addr"`
	Protocol   string `json:"protocol"`
}

func ForwardContextToDataplane(sessionID string, masterKey []byte, listenAddr, remoteAddr, protocol string) error {
	conn, err := net.Dial("tcp", "127.0.0.1:49151")
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx := SessionContext{
		SessionID:  sessionID,
		CryptoKey:  hex.EncodeToString(masterKey),
		ListenAddr: listenAddr,
		RemoteAddr: remoteAddr,
		Protocol:   protocol,
	}

	payload, err := json.Marshal(ctx)
	if err != nil {
		return err
	}

	_, err = conn.Write(payload)
	return err
}
