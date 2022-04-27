package grpcSimple

// Modified from: https://github.com/Dreamacro/clash/blob/master/transport/gun/gun.go
// License: MIT

import (
	"bufio"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
)

var (
	ErrInvalidLength = errors.New("invalid length")
)

var defaultClientHeader = http.Header{
	"content-type": []string{"application/grpc"},
	"user-agent":   []string{"grpc-go/1.41.0"},
	"Te":           []string{"trailers"},
}

//implements net.Conn
type ClientConn struct {
	response  *http.Response
	request   *http.Request
	transport *http2.Transport
	writer    *io.PipeWriter
	once      sync.Once
	close     *atomic.Bool
	err       error
	remain    int
	br        *bufio.Reader

	// deadlines
	timeouter

	client *Client
}

type Config struct {
	ServiceName string
	Host        string
}

func (g *ClientConn) handshake() {
	response, err := g.transport.RoundTrip(g.request)
	if err != nil {
		g.err = err
		g.writer.Close()
		return
	}

	if !g.close.Load() {
		//log.Println("response headers", response.Header)

		if ct := response.Header.Get("Content-Type"); ct != "application/grpc" {
			if ce := utils.CanLogWarn("GRPC Client got wrong Content-Type"); ce != nil {
				ce.Write(zap.String("type", ct))
			}

			g.client.cachedTransport = nil

			response.Body.Close()
			return
		}

		g.response = response
		g.br = bufio.NewReader(response.Body)
	} else {

		g.client.cachedTransport = nil

		response.Body.Close()
	}
}

func (g *ClientConn) Read(b []byte) (n int, err error) {

	g.once.Do(g.handshake)

	if g.err != nil {
		return 0, g.err
	}

	if g.remain > 0 {
		size := g.remain
		if len(b) < size {
			size = len(b)
		}

		n, err = io.ReadFull(g.br, b[:size])
		g.remain -= n
		return
	} else if g.response == nil {
		return 0, net.ErrClosed
	}

	// 0x00 grpclength(uint32) 0x0A uleb128 payload
	_, err = g.br.Discard(6)
	if err != nil {
		return 0, err
	}

	protobufPayloadLen, err := binary.ReadUvarint(g.br)
	if err != nil {
		return 0, ErrInvalidLength
	}

	size := int(protobufPayloadLen)
	if len(b) < size {
		size = len(b)
	}

	n, err = io.ReadFull(g.br, b[:size])
	if err != nil {
		return
	}

	remain := int(protobufPayloadLen) - n
	if remain > 0 {
		g.remain = remain
	}

	return n, nil
}

func (g *ClientConn) Write(b []byte) (n int, err error) {
	protobufHeader := [binary.MaxVarintLen64 + 1]byte{0x0A}
	varuintSize := binary.PutUvarint(protobufHeader[1:], uint64(len(b)))
	grpcHeader := make([]byte, 5)
	grpcPayloadLen := uint32(varuintSize + 1 + len(b))
	binary.BigEndian.PutUint32(grpcHeader[1:5], grpcPayloadLen)

	buf := utils.GetBuf()
	defer utils.PutBuf(buf)
	buf.Write(grpcHeader)
	buf.Write(protobufHeader[:varuintSize+1])
	buf.Write(b)

	_, err = g.writer.Write(buf.Bytes())
	if err == io.ErrClosedPipe && g.err != nil {
		err = g.err
	}
	if err != nil {
		g.client.dealErr(err)

	}

	return len(b), err
}

func (g *ClientConn) Close() error {
	g.close.Store(true)
	if r := g.response; r != nil {
		r.Body.Close()
	}

	return g.writer.Close()
}

const tlsTimeout = time.Second * 5

type Client struct {
	Config

	curBaseConn net.Conn //一般为 tlsConn

	theRequest http.Request

	cachedTransport *http2.Transport

	path string
}

func (g *Client) dealErr(err error) {
	//use of closed connection
	if strings.Contains(err.Error(), "use of closed") {
		g.cachedTransport = nil
	}
}

func (c *Client) GetPath() string {
	return c.ServiceName
}

func (c *Client) IsSuper() bool {
	return false
}

func (c *Client) IsMux() bool {
	return true
}

func (c *Client) IsEarly() bool {
	return false
}

// 由于 本包应用了 http2包, 无法获取特定连接, 所以返回 underlay 本身
func (c *Client) GetCommonConn(underlay net.Conn) (any, error) {

	if underlay == nil {
		if c.cachedTransport != nil {
			return c.cachedTransport, nil
		} else {
			return nil, nil
		}
	} else {
		return underlay, nil

	}
}

func (c *Client) ProcessWhenFull(underlay any) {}

func (c *Client) DialSubConn(underlay any) (net.Conn, error) {

	if underlay == nil {
		return nil, utils.ErrNilParameter
	}

	var transport *http2.Transport

	if t, ok := underlay.(*http2.Transport); ok && t != nil {
		transport = t
	} else {
		transport = &http2.Transport{
			DialTLS: func(_, _ string, cfg *tls.Config) (net.Conn, error) {
				return underlay.(net.Conn), nil
			},
			AllowHTTP:          false,
			DisableCompression: true,
			PingTimeout:        0,
		}
		c.cachedTransport = transport
	}

	reader, writer := io.Pipe()

	request := c.theRequest
	request.Body = reader

	conn := &ClientConn{
		request:   &request,
		transport: transport,
		writer:    writer,
		close:     atomic.NewBool(false),
		client:    c,
	}
	conn.timeouter = timeouter{
		closeFunc: func() {
			conn.Close()
		},
	}

	go conn.once.Do(conn.handshake) //necessary

	return conn, nil
}
