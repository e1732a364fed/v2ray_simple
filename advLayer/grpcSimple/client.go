package grpcSimple

// Modified from: https://github.com/Dreamacro/clash/blob/master/transport/gun/gun.go
// License: MIT

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/atomic"
	"golang.org/x/net/http2"
)

var (
	ErrInvalidLength = errors.New("invalid length")
)

var defaultHeader = http.Header{
	"content-type": []string{"application/grpc"},
	"user-agent":   []string{"grpc-go/1.36.0"},
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
	deadline *time.Timer
}

type Config struct {
	ServiceName string
	Host        string
}

func (g *ClientConn) initRequest() {
	response, err := g.transport.RoundTrip(g.request)
	if err != nil {
		g.err = err
		g.writer.Close()
		return
	}

	if !g.close.Load() {
		g.response = response
		g.br = bufio.NewReader(response.Body)
	} else {
		response.Body.Close()
	}
}

func (g *ClientConn) Read(b []byte) (n int, err error) {
	g.once.Do(g.initRequest)
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

	return len(b), err
}

func (g *ClientConn) Close() error {
	g.close.Store(true)
	if r := g.response; r != nil {
		r.Body.Close()
	}

	return g.writer.Close()
}

func (g *ClientConn) LocalAddr() net.Addr                { return nil }
func (g *ClientConn) RemoteAddr() net.Addr               { return nil }
func (g *ClientConn) SetReadDeadline(t time.Time) error  { return g.SetDeadline(t) }
func (g *ClientConn) SetWriteDeadline(t time.Time) error { return g.SetDeadline(t) }

func (g *ClientConn) SetDeadline(t time.Time) error {
	d := time.Until(t)
	if g.deadline != nil {
		g.deadline.Reset(d)
		return nil
	}
	g.deadline = time.AfterFunc(d, func() {
		g.Close()
	})
	return nil
}

const tlsTimeout = time.Second * 5

func NewHTTP2Client(
	rawTCPConn net.Conn,
	tlsConfig *tls.Config,
) *http2.Transport {

	dialFunc := func(_, _ string, cfg *tls.Config) (net.Conn, error) {

		cn := tls.Client(rawTCPConn, cfg)

		ctx, cancel := context.WithTimeout(context.Background(), tlsTimeout)
		defer cancel()
		if err := cn.HandshakeContext(ctx); err != nil {
			rawTCPConn.Close()
			return nil, err
		}
		state := cn.ConnectionState()
		if p := state.NegotiatedProtocol; p != http2.NextProtoTLS {
			cn.Close()
			return nil, utils.ErrInErr{
				ErrDesc:   "grpcHardcore, http2: unexpected ALPN protocol",
				ErrDetail: utils.ErrInvalidData,
				Data:      p,
			}
		}
		return cn, nil
	}

	return &http2.Transport{
		DialTLS:            dialFunc,
		TLSClientConfig:    tlsConfig,
		AllowHTTP:          false,
		DisableCompression: true,
		PingTimeout:        0,
	}
}

func StreamGunWithTransport(transport *http2.Transport, cfg *Config) (net.Conn, error) {
	serviceName := "GunService"
	if cfg.ServiceName != "" {
		serviceName = cfg.ServiceName
	}

	reader, writer := io.Pipe()
	request := &http.Request{
		Method: http.MethodPost,
		Body:   reader,
		URL: &url.URL{
			Scheme: "https",
			Host:   cfg.Host,
			Path:   fmt.Sprintf("/%s/Tun", serviceName),
			// for unescape path
			Opaque: fmt.Sprintf("//%s/%s/Tun", cfg.Host, serviceName),
		},
		Proto:      "HTTP/2",
		ProtoMajor: 2,
		ProtoMinor: 0,
		Header:     defaultHeader,
	}

	conn := &ClientConn{
		request:   request,
		transport: transport,
		writer:    writer,
		close:     atomic.NewBool(false),
	}

	go conn.once.Do(conn.initRequest)
	return conn, nil
}

func GetNewClientStream(conn net.Conn, tlsConfig *tls.Config, cfg *Config) (net.Conn, error) {

	transport := NewHTTP2Client(conn, tlsConfig)
	return StreamGunWithTransport(transport, cfg)
}

func GetNewClientStream_withTlsConn(conn net.Conn, cfg *Config) (net.Conn, error) {

	transport := &http2.Transport{
		DialTLS: func(_, _ string, cfg *tls.Config) (net.Conn, error) {
			return conn, nil
		},
		AllowHTTP:          false,
		DisableCompression: true,
		PingTimeout:        0,
	}
	return StreamGunWithTransport(transport, cfg)
}
