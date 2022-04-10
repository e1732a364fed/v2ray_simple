package proxy

import (
	"crypto/tls"
	"log"
	"net"
	"net/url"

	"github.com/hahahrfool/v2ray_simple/advLayer/quic"
	"github.com/hahahrfool/v2ray_simple/httpLayer"
	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/tlsLayer"
	"github.com/hahahrfool/v2ray_simple/utils"
	"go.uber.org/zap"
)

//use dc.Host, dc.Insecure, dc.Utls
// 如果用到了quic，还会直接配置quic的client的所有设置.
func prepareTLS_forClient(com ProxyCommon, dc *DialConf) error {
	alpnList := dc.Alpn

	switch com.AdvancedLayer() {
	case "quic":
		na, e := netLayer.NewAddr(com.AddrStr())
		if e != nil {
			if ce := utils.CanLogErr("prepareTLS_forClient,quic,netLayer.NewAddr failed"); ce != nil {
				ce.Write(zap.Error(e))
			}
			return e
		}

		com.setNetwork("udp")
		var useHysteria, hysteria_manual bool
		var maxbyteCount int

		if dc.Extra != nil {
			if thing := dc.Extra["congestion_control"]; thing != nil {
				if use, ok := thing.(string); ok && use == "hy" {
					useHysteria = true

					if thing := dc.Extra["mbps"]; thing != nil {
						if mbps, ok := thing.(int64); ok && mbps > 1 {
							maxbyteCount = int(mbps) * 1024 * 1024 / 8

							log.Println("Using Hysteria Congestion Control, max upload mbps: ", mbps)
						}
					} else {
						log.Println("Using Hysteria Congestion Control, max upload mbps: ", quic.Default_hysteriaMaxByteCount, "mbps")
					}

					if thing := dc.Extra["hy_manual"]; thing != nil {
						if ismanual, ok := thing.(bool); ok {
							hysteria_manual = ismanual
							if ismanual {
								log.Println("Using Hysteria Manual Control Mode")
							}
						}
					}
				}
			}

		}

		if len(alpnList) == 0 {
			alpnList = quic.AlpnList
		}

		com.setQuic_Client(quic.NewClient(&na, alpnList, dc.Host, dc.Insecure, useHysteria, maxbyteCount, hysteria_manual))
		return nil //quic直接接管了tls，所以不执行下面步骤

	case "grpc":
		has_h2 := false
		for _, a := range alpnList {
			if a == httpLayer.H2_Str {
				has_h2 = true
				break
			}
		}
		if !has_h2 {
			alpnList = append([]string{httpLayer.H2_Str}, alpnList...)
		}
	}
	com.setTLS_Client(tlsLayer.NewClient(dc.Host, dc.Insecure, dc.Utls, alpnList))
	return nil
}

//use lc.Host, lc.TLSCert, lc.TLSKey, lc.Insecure
// 如果用到了quic，还会直接配置quic的server的所有设置.
func prepareTLS_forServer(com ProxyCommon, lc *ListenConf) error {
	// 这里直接不检查 字符串就直接传给 tlsLayer.NewServer
	// 所以要求 cert和 key 不在程序本身目录 的话，就要给出完整路径

	alpnList := lc.Alpn
	switch com.AdvancedLayer() {
	case "quic":

		com.setNetwork("udp")

		if len(alpnList) == 0 {
			alpnList = quic.AlpnList
		}

		var useHysteria bool
		var hysteria_manual bool
		var maxbyteCount int
		var maxStreamCountInOneSession int64

		if lc.Extra != nil {

			if thing := lc.Extra["maxStreamCountInOneSession"]; thing != nil {
				if count, ok := thing.(int64); ok && count > 0 {
					log.Println("maxStreamCountInOneSession,", count)
					maxStreamCountInOneSession = count

				}

			}

			if thing := lc.Extra["congestion_control"]; thing != nil {
				if use, ok := thing.(string); ok && use == "hy" {
					useHysteria = true

					if thing := lc.Extra["mbps"]; thing != nil {
						if mbps, ok := thing.(int64); ok && mbps > 1 {
							maxbyteCount = int(mbps) * 1024 * 1024 / 8

							log.Println("Using Hysteria Congestion Control, max upload mbps: ", mbps)

						}
					} else {

						log.Println("Using Hysteria Congestion Control, max upload mbps:", quic.Default_hysteriaMaxByteCount, "mbps")

					}

					if thing := lc.Extra["hy_manual"]; thing != nil {
						if ismanual, ok := thing.(bool); ok {
							hysteria_manual = ismanual
							if ismanual {
								log.Println("Using Hysteria Manual Control Mode")
							}
						}
					}
				}
			}

		}

		com.setListenCommonConnFunc(func() (newConnChan chan net.Conn, baseConn any) {

			certArray, err := tlsLayer.GetCertArrayFromFile(lc.TLSCert, lc.TLSKey)

			if err != nil {

				if ce := utils.CanLogErr("can't create tls cert"); ce != nil {
					ce.Write(zap.String("cert", lc.TLSCert), zap.String("key", lc.TLSKey), zap.Error(err))
				}

				return nil, nil
			}

			return quic.ListenInitialLayers(com.AddrStr(), tls.Config{
				InsecureSkipVerify: lc.Insecure,
				ServerName:         lc.Host,
				Certificates:       certArray,
				NextProtos:         alpnList,
			}, useHysteria, maxbyteCount, hysteria_manual, maxStreamCountInOneSession)

		})

		return nil //quic直接接管了tls，所以不执行下面步骤
	case "grpc":
		has_h2 := false
		for _, a := range alpnList {
			if a == httpLayer.H2_Str {
				has_h2 = true
				break
			}
		}
		if !has_h2 {
			alpnList = append([]string{httpLayer.H2_Str}, alpnList...)
		}
	}

	tlsserver, err := tlsLayer.NewServer(lc.Host, lc.TLSCert, lc.TLSKey, lc.Insecure, alpnList)
	if err == nil {
		com.setTLS_Server(tlsserver)
	} else {
		return err
	}
	return nil
}

//给 ProxyCommon 的tls做一些配置上的准备，从url读取配置
func prepareTLS_forProxyCommon_withURL(u *url.URL, isclient bool, com ProxyCommon) error {
	insecureStr := u.Query().Get("insecure")
	insecure := false
	if insecureStr != "" && insecureStr != "false" && insecureStr != "0" {
		insecure = true
	}

	if isclient {
		utlsStr := u.Query().Get("utls")
		useUtls := utlsStr != "" && utlsStr != "false" && utlsStr != "0"
		com.setTLS_Client(tlsLayer.NewClient(u.Host, insecure, useUtls, nil))

	} else {
		certFile := u.Query().Get("cert")
		keyFile := u.Query().Get("key")

		hostAndPort := u.Host
		sni, _, _ := net.SplitHostPort(hostAndPort)

		tlsserver, err := tlsLayer.NewServer(sni, certFile, keyFile, insecure, nil)
		if err == nil {
			com.setTLS_Server(tlsserver)
		} else {
			return err
		}
	}
	return nil
}
