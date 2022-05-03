package grpcSimple

import (
	"bufio"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/e1732a364fed/v2ray_simple/httpLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
)

var defaultClientHeader = http.Header{
	"content-type": []string{grpcContentType},
	"user-agent":   []string{"grpc-go/1.41.0"},
	"Te":           []string{"trailers"},
}

type Config struct {
	ServiceName string
	Host        string

	FallbackToH1 bool //默认会回落到h2, 如果指定 FallbackToH1， 则会回落到 http/1.1
}

//implements net.Conn
type ClientConn struct {
	commonPart
	timeouter

	client *Client

	response      *http.Response
	request       *http.Request
	transport     *http2.Transport
	writer        *io.PipeWriter
	handshakeOnce sync.Once
	shouldClose   *atomic.Bool
	err           error
}

func (g *ClientConn) handshake() {
	response, err := g.transport.RoundTrip(g.request)
	if err != nil {
		g.err = err
		g.writer.Close()
		return
	}

	notOK := false

	if g.shouldClose.Load() {
		notOK = true
	} else {
		//log.Println("response headers", response.Header)

		if ct := response.Header.Get("Content-Type"); ct != "application/grpc" {
			if ce := utils.CanLogWarn("GRPC Client got wrong Content-Type"); ce != nil {
				ce.Write(zap.String("type", ct))
			}

			notOK = true
		} else if g.client != nil && len(g.client.responseHeader) > 0 {
			if ok, firstNotMatchKey := httpLayer.AllHeadersIn(g.client.responseHeader, response.Header); !ok {

				if ce := utils.CanLogWarn("GRPC Client configured custom header, but the server response doesn't have all of them"); ce != nil {
					ce.Write(zap.String("firstNotMatchKey", firstNotMatchKey))
				}

				notOK = true
			}
		}

	}

	if notOK {

		g.client.cachedTransport = nil

		response.Body.Close()
	} else {
		g.response = response
		g.br = bufio.NewReader(response.Body)
	}
}

func (g *ClientConn) Read(b []byte) (n int, err error) {

	g.handshakeOnce.Do(g.handshake)

	if g.err != nil {
		return 0, g.err
	}

	if g.response == nil {
		return 0, net.ErrClosed
	}

	return g.commonPart.Read(b)

}

func (g *ClientConn) Write(b []byte) (n int, err error) {

	buf := commonWrite(b)
	_, err = g.writer.Write(buf.Bytes())
	utils.PutBuf(buf)

	if err == io.ErrClosedPipe && g.err != nil {
		err = g.err
	}
	if err != nil {
		g.client.dealErr(err)

	}

	return len(b), err
}

func (g *ClientConn) Close() error {
	g.shouldClose.Store(true)
	if r := g.response; r != nil {
		r.Body.Close()
	}

	return g.writer.Close()
}

type Client struct {
	Creator

	Config

	curBaseConn net.Conn //一般为 tlsConn

	handshakeRequest http.Request

	responseHeader map[string][]string

	cachedTransport *http2.Transport //一个 transport 对应 一个提供的 dial好的 tls 连接，正好作为CommonConn。

	path string
}

func (g *Client) dealErr(err error) {
	//use of closed connection

	if errors.Is(err, net.ErrClosed) {
		g.cachedTransport = nil
	} else if strings.Contains(err.Error(), "use of closed") {
		g.cachedTransport = nil
	}
}

func (c *Client) GetPath() string {
	return c.ServiceName
}

func (c *Client) IsEarly() bool {
	return false
}

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

	request := c.handshakeRequest
	request.Body = reader

	conn := &ClientConn{
		request:     &request,
		transport:   transport,
		writer:      writer,
		shouldClose: atomic.NewBool(false),
		client:      c,
	}
	conn.timeouter = timeouter{
		closeFunc: func() {
			conn.Close()
		},
	}

	go conn.handshakeOnce.Do(conn.handshake) //necessary

	return conn, nil
}
