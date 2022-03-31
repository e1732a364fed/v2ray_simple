package tlsLayer_test

import (
	"bytes"
	"io"
	"io/ioutil"
	"net"
	"testing"

	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/proxy"
	_ "github.com/hahahrfool/v2ray_simple/proxy/vless"
	"github.com/hahahrfool/v2ray_simple/utils"
)

func TestVlesss(t *testing.T) {
	testTls("vlesss", netLayer.RandPortStr(), t)
}

func testTls(protocol string, port string, t *testing.T) {
	if !utils.FileExist("../cert.pem") {
		ioutil.WriteFile("../cert.pem", []byte(sampleCertStr), 0777)
	}

	if !utils.FileExist("../cert.key") {
		ioutil.WriteFile("../cert.key", []byte(sampleKeyStr), 0777)
	}

	url := protocol + "://a684455c-b14f-11ea-bf0d-42010aaa0003@localhost:" + port + "?alterID=4&cert=../cert.pem&key=../cert.key&insecure=1"
	server, hase, errx := proxy.ServerFromURL(url)
	if hase {
		t.Log("fail1", errx)
		t.FailNow()
	}
	defer server.Stop()
	client, hase, errx := proxy.ClientFromURL(url)
	if hase {
		t.Log("fail2", errx)
		t.FailNow()
	}

	targetStr := "dummy.com:80"
	targetStruct := &netLayer.Addr{
		Name: "dummy.com",
		Port: 80,
	}

	listener, err := net.Listen("tcp", server.AddrStr())
	if err != nil {
		t.Logf("can not listen on %v: %v", server.AddrStr(), err)
		t.FailNow()
	}
	go func() {

		lc, err := listener.Accept()
		if err != nil {
			t.Logf("failed in accept: %v", err)
			t.Fail()
			return
		}
		go func() {
			defer lc.Close()

			t.Log("server got new Conn")

			tlsConn, err := server.GetTLS_Server().Handshake(lc)
			if err != nil {
				t.Log("failed in server tls handshake  ", err)
				t.Fail()
				return
			}
			lc = tlsConn

			t.Log("server pass tls handshake")

			wlc, targetAddr, err := server.Handshake(lc)
			if err != nil {
				t.Log("failed in handshake from ", server.AddrStr(), err)
				t.Fail()
				return
			}

			t.Log("server pass vless handshake")

			if targetAddr.String() != targetStr {
				t.Fail()
				return
			}

			var hello [5]byte
			io.ReadFull(wlc, hello[:])
			if !bytes.Equal(hello[:], []byte("hello")) {
				t.Fail()
				return
			}

			wlc.Write([]byte("world"))
		}()

	}()

	t.Log("client dial ")

	rc, _ := net.Dial("tcp", server.AddrStr())
	defer rc.Close()

	t.Log("client handshake tls ")

	tlsConn, err := client.GetTLS_Client().Handshake(rc)
	if err != nil {
		t.Log("failed in client tls handshake  ", err)
		t.FailNow()
	}
	rc = tlsConn

	t.Log("client handshake vless ")

	wrc, err := client.Handshake(rc, targetStruct)
	if err != nil {
		t.Log("failed in handshake to", server.AddrStr(), err)
		t.FailNow()
	}

	t.Log("client write hello ")

	wrc.Write([]byte("hello"))

	t.Log("client read response ")
	var world [5]byte
	io.ReadFull(wrc, world[:])
	if !bytes.Equal(world[:], []byte("world")) {
		t.FailNow()
	}
}

var sampleCertStr string = `
-----BEGIN CERTIFICATE-----
MIIDRjCCAi4CCQCSSzLVNZv+qDANBgkqhkiG9w0BAQsFADBlMQswCQYDVQQGEwJj
bjEMMAoGA1UECAwDc2RmMQwwCgYDVQQHDANhc2YxCzAJBgNVBAoMAnNmMQswCQYD
VQQLDAJkZjEMMAoGA1UEAwwDZGZzMRIwEAYJKoZIhvcNAQkBFgNzZmQwHhcNMjIw
MzAzMTczMDIyWhcNMjIwNDAyMTczMDIyWjBlMQswCQYDVQQGEwJjbjEMMAoGA1UE
CAwDc2RmMQwwCgYDVQQHDANhc2YxCzAJBgNVBAoMAnNmMQswCQYDVQQLDAJkZjEM
MAoGA1UEAwwDZGZzMRIwEAYJKoZIhvcNAQkBFgNzZmQwggEiMA0GCSqGSIb3DQEB
AQUAA4IBDwAwggEKAoIBAQCpbSsu01VAR6nehexyyGJmcl05p9bE/OB7ZNqJwc/o
8xu+5wyYg102AYGPGvvolCSr+vIOsCrGwmlLE7tlJvJk2NUBlBhEgRIgHRM5mwqr
9P380Lat7qaCMQ90MuN3kXeKYYp8Wp+5qBG3YI7KdtGMvycLnncWFjR1U6LE3sxj
yDy12+57FYV5IH6suS9cxGUYgddLkrN0mk8qTDTuEa2Ks7pBlWDFbnY/cUkaecYM
0e5F3vRAJJt3n1dIgZ/aM1p1dLS9uJoBiSfTbAsytkVB9eYLRcMWhtFeNyN4hs3/
CPKg3pNsIzUuFz4v0eVTN7FjSDIveHdppIw8qFKX1K8dAgMBAAEwDQYJKoZIhvcN
AQELBQADggEBAJw/UdmBc3mqpaTfy8ZlelTKd8vQvEXiNMU/A9ie7dIyUxYQufNw
jLYUJs1WEY5oDjA7zRsF0UGxYrIrx3zSsacw5VGOjjHFSSu2ZTJ7glFufnWlfEl9
MBET2sHFg9vm35H0aKvwMEQppezwfO/YC9qAzm6Vl3R/pEvbxxSfdaqY02hkfAiu
t0IUzm1AWAU/KJyKjVQnlvKH0rc59lAplWOul1Ju0mhgbbXt2BfLh2TpcRvvaver
shPR8VTaQbmssaTAu0TEI5tBODkB74c+zEO4lHbtsc3vtO1os/NYnL7S27Cjt7zI
j5cpQ75Kw4zg8qeTwHqnQmo4z8sGfEUm1kI=
-----END CERTIFICATE-----
`

var sampleKeyStr string = `
-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAqW0rLtNVQEep3oXscshiZnJdOafWxPzge2TaicHP6PMbvucM
mINdNgGBjxr76JQkq/ryDrAqxsJpSxO7ZSbyZNjVAZQYRIESIB0TOZsKq/T9/NC2
re6mgjEPdDLjd5F3imGKfFqfuagRt2COynbRjL8nC553FhY0dVOixN7MY8g8tdvu
exWFeSB+rLkvXMRlGIHXS5KzdJpPKkw07hGtirO6QZVgxW52P3FJGnnGDNHuRd70
QCSbd59XSIGf2jNadXS0vbiaAYkn02wLMrZFQfXmC0XDFobRXjcjeIbN/wjyoN6T
bCM1Lhc+L9HlUzexY0gyL3h3aaSMPKhSl9SvHQIDAQABAoIBAEHREexv1ndRH5E9
L1xrsaYgmUyTgeAmaEInLKpFKzJQdp/Te9YneedH8H+aOO/h1NkmdC/2ibeKwIKU
2MBzv8gjX6PsVv0Nsu/cu6IuM5gXZS94GO86fV6oFlvKhQjm7qxINhcW0WO7AZ7e
GLpYLBFkFJPz7EkdOSW25s1Zy8aa2gDco2QjywxeJkUhk1ewtIW6x7yL9KlXwb/V
ewtmbnlPIAE16WoKRe3lSfJatffkEU9uJsGVVxvfow6sJ2AQBZ0D3zBdJYgLupoX
B+UaTYe3Upb4cma3EAJLXcUW9crmYoPH4OW8tsHzi1TlUg9eQd0DgEAcVcM3A+I9
7lgyX1UCgYEA0rgiuiUzpCy+y8c2Mi92QiOdaxBfP4WBfi9HbXjU83H8SpT358cn
JQfvZVMcRm3zTZFe5Z0mlTJOAs1uj9McrDP1p48k8ahoGYAtSm6etFTRR0Kq12PA
32fUqXV81i5iGwmdSfmWfeAuY5XbJ38xcRYOeHOV0/G2w1qwCOeUJjsCgYEAzdV5
sSzf6EZybK/guHIdHUqCxTAc7T02h3CImz7iEaGawU+AzGc1q5zvEAk90iEGskl0
vjTTFL+VamsicmW3P5DZN0/Gut5FE/AaTWjG8hyGhsS2npoiA9jtvjaD4XQKNI0a
7clU356muhPwasiwVe2Vx6eMJLb/mhKCUn7nMocCgYEAu6Pa0LXF+aEaua2IlkHr
ZdP/HtKyboc9G5eQXGxn/Oz4w5VJ+GxAcFpTlH/gwtqv+NfFkGRTcjIcg6RZmttc
Qf/29aGjPUpAgMzCB/DfhCevQGyeYzTiEE6OceQ8KSGenQL/vFrz5t1VkbplMBO0
fEYu1pXeyqAIpodAEH3fT/cCgYB1qtjzgTzLEyKsoWqs5odgTE0vns6ajMjUam+d
mDgybhkC84kk0MeswH0lxLKzoi+q0jVL2vTkQpWPDYnWrfExBIQ4i4GHKDODL1pJ
8GDy3X3GI0RmrKRPYL6gY5fG1chTvGqtjs/XOmIDtAxXbzznEnfyeAS0pGzATl5z
/Jn8lwKBgA3saY5JtIREaRRBdVuIi+lWT8wDwoXWx7oQoVY4yTqVCsxF3GlteLnA
S+tJdmjCPCOigsikWNxiJtPSYOeQCHZhyU0eS9/oiLWi51BElote3lzCp3J6ePmt
cJ3yJB1V/DcTOGm/zxInGkFhcY8lYfFbrWWmifbo2wl5GcIJpaT2
-----END RSA PRIVATE KEY-----
`
