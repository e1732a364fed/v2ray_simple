package vless

import (
	"net"

	"github.com/hahahrfool/v2ray_simple/proxy"
)

const Name = "vless"

const (
	Cmd_CRUMFURS byte = 4 // start from vless v1

	CRUMFURS_ESTABLISHED byte = 20

	CRUMFURS_Established_Str = "CRUMFURS_Established"
)

type UserConn struct {
	net.Conn
	uuid         [16]byte
	convertedStr string
	version      int
	isUDP        bool
}

func (uc *UserConn) GetProtocolVersion() int {
	return uc.version
}
func (uc *UserConn) GetIdentityStr() string {
	if uc.convertedStr == "" {
		uc.convertedStr = proxy.UUIDToStr(uc.uuid)
	}

	return uc.convertedStr
}

func (uc *UserConn) Read(p []byte) (int, error) {
	if uc.version == 0 && uc.isUDP {
		return uc.Conn.Read(p)
	} else {
		return uc.Conn.Read(p)

	}
}

func (uc *UserConn) Write(p []byte) (int, error) {
	if uc.version == 0 && uc.isUDP {
		return uc.Conn.Write(p)
	} else {
		return uc.Conn.Write(p)

	}
}
