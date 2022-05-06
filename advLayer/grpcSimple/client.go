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

func (c *ClientConn) handshake() {
	response, err := c.transport.RoundTrip(c.request)
	if err != nil {
		c.err = err
		c.writer.Close()
		return
	}

	notOK := false

	if c.shouldClose.Load() {
		notOK = true
	} else {
		//log.Println("response headers", response.Header)

		if ct := response.Header.Get("Content-Type"); ct != "application/grpc" {
			if ce := utils.CanLogWarn("GRPC Client got wrong Content-Type"); ce != nil {
				ce.Write(zap.String("type", ct))
			}

			notOK = true
		} else if c.client != nil && len(c.client.responseHeader) > 0 {
			if ok, firstNotMatchKey := httpLayer.AllHeadersIn(c.client.responseHeader, response.Header); !ok {

				if ce := utils.CanLogWarn("GRPC Client configured custom header, but the server response doesn't have all of them"); ce != nil {
					ce.Write(zap.String("firstNotMatchKey", firstNotMatchKey))
				}

				notOK = true
			}
		}

	}

	if notOK {

		c.client.cachedTransport = nil

		response.Body.Close()
	} else {
		c.response = response
		c.br = bufio.NewReader(response.Body)
	}
}

func (c *ClientConn) Read(b []byte) (n int, err error) {

	c.handshakeOnce.Do(c.handshake)

	if c.err != nil {
		return 0, c.err
	}

	if c.response == nil {
		return 0, net.ErrClosed
	}

	return c.commonPart.Read(b)

}

func (c *ClientConn) Write(b []byte) (n int, err error) {

	buf := commonWrite(b)
	_, err = c.writer.Write(buf.Bytes())
	utils.PutBuf(buf)

	if err == io.ErrClosedPipe && c.err != nil {
		err = c.err
	}
	if err != nil {
		c.client.dealErr(err)

	}

	return len(b), err
}

func (c *ClientConn) Close() error {
	c.shouldClose.Store(true)
	if r := c.response; r != nil {
		r.Body.Close()
	}

	return c.writer.Close()
}

//implements advLayer.MuxClient
type Client struct {
	Creator

	Config

	curBaseConn net.Conn //一般为 tlsConn

	handshakeRequest http.Request

	responseHeader map[string][]string

	cachedTransport *http2.Transport //一个 transport 对应 一个提供的 dial好的 tls 连接，正好作为CommonConn。

	path string
}

func (c *Client) dealErr(err error) {
	//use of closed connection

	if errors.Is(err, net.ErrClosed) {
		c.cachedTransport = nil
	} else if strings.Contains(err.Error(), "use of closed") {
		c.cachedTransport = nil
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
