package proxy

import (
	"io"
	"net"
)

// 阻塞
func RelayTCP(selfLocalServerConn, selfRemoteConn net.Conn) {
	go io.Copy(selfRemoteConn, selfLocalServerConn)
	io.Copy(selfLocalServerConn, selfRemoteConn)
}

// 阻塞.
func RelayUDP(putter UDP_Putter, extractor UDP_Extractor) {

	go func() {
		for {
			raddr, bs, err := extractor.GetNewUDPRequest()
			if err != nil {
				break
			}
			err = putter.WriteUDPRequest(raddr, bs)
			if err != nil {
				break
			}
		}
	}()

	for {
		raddr, bs, err := putter.GetNewUDPResponse()
		if err != nil {
			break
		}
		err = extractor.WriteUDPResponse(raddr, bs)
		if err != nil {
			break
		}
	}
}
