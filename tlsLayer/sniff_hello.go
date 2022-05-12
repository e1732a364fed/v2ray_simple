package tlsLayer

import (
	"crypto/tls"
	"log"
)

//parse rand, session id, cipher_suites, compression_methods, return bytes after compression_methods.
func (cd *ComSniff) sniff_commonHelloPre(pAfter []byte) []byte {
	pAfterRand := pAfter[32:]
	sessionL := pAfterRand[0]

	if 1+int(sessionL) > len(pAfterRand) {
		cd.DefinitelyNotTLS = true
		cd.handshakeFailReason = 7
		return nil
	}

	pAfterSessionID := pAfterRand[1+sessionL:]

	cipher_suitesLen := uint16(pAfterSessionID[1]) | uint16(pAfterSessionID[0])<<8

	if 2+int(cipher_suitesLen) > len(pAfterSessionID) {
		cd.DefinitelyNotTLS = true
		cd.handshakeFailReason = 8
		return nil
	}

	pAfterCipherSuites := pAfterSessionID[2+cipher_suitesLen:]

	legacy_compression_methodsLen := pAfterCipherSuites[0]

	/*
		legacy_compression_methods:  Versions of TLS before 1.3 supported
		compression with the list of supported compression methods being
		sent in this field.  For every TLS 1.3 ClientHello, this vector
		MUST contain exactly one byte, set to zero, which corresponds to
		the "null" compression method in prior versions of TLS.  If a
		TLS 1.3 ClientHello is received with any other value in this
		field, the server MUST abort the handshake with an
		"illegal_parameter" alert.  Note that TLS 1.3 servers might
		receive TLS 1.2 or prior ClientHellos which contain other
		compression methods and (if negotiating such a prior version) MUST
		follow the procedures for the appropriate prior version of TLS.

		然后对于tls1.3来说，服务端的这一项也必须是0
	*/

	if 1+int(legacy_compression_methodsLen) > len(pAfterCipherSuites) {
		cd.DefinitelyNotTLS = true
		cd.handshakeFailReason = 9
		return nil
	}
	pAfterLegacy_compression_methods := pAfterCipherSuites[1+legacy_compression_methodsLen:]

	if len(pAfterLegacy_compression_methods) == 0 {
		//没有多余字节，则表明该连接肯定是tls1.2

		if PDD {
			log.Println("R No extension, Definitely tls1.2", len(pAfterLegacy_compression_methods))
		}
		cd.helloPacketPass = true
		cd.CantBeTLS13 = true
		return nil
	}

	if len(pAfterLegacy_compression_methods) == 1 {
		//有多余字节，似乎是tls1.3, 但是信息却不满足 tls1.3 要求，那么就是不合法的

		cd.DefinitelyNotTLS = true
		cd.handshakeFailReason = 10
		return nil
	}

	return pAfterLegacy_compression_methods

}

//需要判断到底是 tls 1.3 还是 tls1.2。
//可参考 https://halfrost.com/https_tls1-3_handshake/ 。
// 具体见最上面的注释，以及rfc。
//解析还可以参考 https://blog.csdn.net/weixin_36139431/article/details/103541874
func (cd *ComSniff) sniff_hello(pAfter []byte, isclienthello bool, onlyForSni bool) {
	pAfterLegacy_compression_methods := cd.sniff_commonHelloPre(pAfter)

	if cd.helloPacketPass || cd.DefinitelyNotTLS {
		return
	}

	extensionsLen := uint16(pAfterLegacy_compression_methods[1]) | uint16(pAfterLegacy_compression_methods[0])<<8

	//log.Println("extensionsLen", extensionsLen)

	if extensionsLen < 8 {
		//有多余字节，看似是 tls1.3, 但是信息却不满足 tls1.3 要求，那么就是不合法的

		cd.DefinitelyNotTLS = true
		cd.handshakeFailReason = 11
	}

	if len(pAfterLegacy_compression_methods) < 2+int(extensionsLen) {

		//如果长度大于应有的长度，也是可能的，因为 tls1.3 的 0-rtt, 所以只有小于该长度的是非法的
		// 然而，

		if PDD {
			log.Println("R 1+int(extensionsLen)+8 < len(pAfterLegacy_compression_methods)", 1+int(extensionsLen)+8, len(pAfterLegacy_compression_methods))
		}

		cd.DefinitelyNotTLS = true
		cd.handshakeFailReason = 12
		return
	}

	/*
		然后就开始判断extension了

		struct {
			ExtensionType extension_type;
			opaque extension_data<0..2^16-1>;
		} Extension;

		所有extension 列表：
			https://www.iana.org/assignments/tls-extensiontype-values/tls-extensiontype-values.xhtml

		enum {
			server_name(0),                              RFC 6066
			max_fragment_length(1),                      RFC 6066
			status_request(5),                           RFC 6066
			supported_groups(10),                        RFC 8422, 7919
			signature_algorithms(13),                    RFC 8446
			use_srtp(14),                                RFC 5764
			heartbeat(15),                               RFC 6520
			application_layer_protocol_negotiation(16),  RFC 7301
			signed_certificate_timestamp(18),            RFC 6962
			client_certificate_type(19),                 RFC 7250
			server_certificate_type(20),                 RFC 7250
			padding(21),                                 RFC 7685
			pre_shared_key(41),                          RFC 8446
			early_data(42),                              RFC 8446
			supported_versions(43),                      RFC 8446
			cookie(44),                                  RFC 8446
			psk_key_exchange_modes(45),                  RFC 8446
			certificate_authorities(47),                 RFC 8446
			oid_filters(48),                             RFC 8446
			post_handshake_auth(49),                     RFC 8446
			signature_algorithms_cert(50),               RFC 8446
			key_share(51),                               RFC 8446
			(65535)
		} ExtensionType;

		没有地方 给出整个Extensions的数量，只能按顺序读取;

		而如果是 0-rtt的情况的话， pre_shared_key 必须是最后一个extension。

		"When multiple extensions of different types are present, the
		extensions MAY appear in any order, with the exception of
		"pre_shared_key" (Section 4.2.11) which MUST be the last extension in
		the ClientHello "

		这样，就算是0-rtt，也能判断出来 Extensions的尾部边界

		还可参考
		https://xiaochai.github.io/2020/07/05/tls/

		https://commandlinefanatic.com/cgi-bin/showarticle.cgi?article=art080


	*/

	extensionsBs := pAfterLegacy_compression_methods[2 : 2+extensionsLen]

	lenE := len(extensionsBs)

	//if PDD {
	//	log.Println("extensionsBs", extensionsBs)
	//}

	cursor := 0
	//虽然我们知道 extensionsBs的总长度 extensionsLen，但是
	// supportedVersions 这个extension的位置是未知的！所以我们必须循环判断，好麻烦啊！
	for cursor < lenE {
		//前两字节是 ExtensionType
		et := uint16(extensionsBs[cursor])<<8 + uint16(extensionsBs[cursor+1])

		//就算extension是未在rfc定义的，也不能就证明是无效的tls，因为整个extension组合是在iana定义的，
		// 而且确实 客户可以自定义 extension，来达到自己想要实现的效果

		cursor += 2

		thiseLen := uint16(extensionsBs[cursor])<<8 + uint16(extensionsBs[cursor+1])

		cursor += 2

		if PDD {
			log.Println("Got Extension:", et, "'", etStrMap[int(et)], "'", "len", thiseLen)
		}

		//我们按照 rfc8446 的文档顺序来进行过滤, 但是首先把 0-21的提到前面来, 因为它们更加常见，尤其是 sni和 alpn

		switch et {
		default:
			cursor += int(thiseLen)

		/////////////////////////////////////////////////////////
		/*
			下列结构，分属各个其他rfc
			server_name(0),                              RFC 6066
			max_fragment_length(1),                      RFC 6066
			status_request(5),                           RFC 6066
			use_srtp(14),                                RFC 5764
			heartbeat(15),                               RFC 6520
			application_layer_protocol_negotiation(16),  RFC 7301
			signed_certificate_timestamp(18),            RFC 6962
			client_certificate_type(19),                 RFC 7250
			server_certificate_type(20),                 RFC 7250
			padding(21),                                 RFC 7685

		*/
		case 0: //server_name, 一般而言，extension是按顺序的，所以大部分情况最前面是这一项
			//https://datatracker.ietf.org/doc/html/rfc6066#section-3
			/*
				struct {
					NameType name_type;
					select (name_type) {
						case host_name: HostName;
					} name;
				} ServerName;
				enum {
					host_name(0), (255)
				} NameType;

				opaque HostName<1..2^16-1>;

				struct {
					ServerName server_name_list<1..2^16-1>
				} ServerNameList;

				一个列表，前面两字节长度， 然后有n个域名提供; 一般用户只会带1个servername。
			*/
			ServerNameListLen := int(extensionsBs[cursor])<<8 + int(extensionsBs[cursor+1])
			cursor += 2
			if len(extensionsBs[cursor:]) < ServerNameListLen {
				cd.DefinitelyNotTLS = true
				cd.handshakeFailReason = 21
				return

			}

			edge := cursor + ServerNameListLen
			sn_count := 0

			for cursor < edge {
				sn_count++

				if extensionsBs[cursor] != 0 {
					cd.DefinitelyNotTLS = true
					cd.handshakeFailReason = 22
					return
				}
				cursor++
				l := int(extensionsBs[cursor])<<8 + int(extensionsBs[cursor+1])
				cursor += 2
				if len(extensionsBs[cursor:]) < l {
					cd.DefinitelyNotTLS = true
					cd.handshakeFailReason = 22
					return
				}

				cd.SniffedServerName = string(extensionsBs[cursor : cursor+l])
				if onlyForSni {
					return
				}
				cursor += l

				if PDD {
					log.Println("cd.SniffedHostName", sn_count, cd.SniffedServerName)
				}

			}

		case 1: //max_fragment_length
			//https://datatracker.ietf.org/doc/html/rfc6066#section-4
			//
			//enum{
			//	2^9(1), 2^10(2), 2^11(3), 2^12(4), (255)
			//	} MaxFragmentLength; 即1字节

			b := uint64(extensionsBs[cursor])
			switch b {
			case 2 << 9:
				fallthrough
			case 2 << 10:
				fallthrough
			case 2 << 11:
				fallthrough
			case 2 << 12:

			default:
				cd.DefinitelyNotTLS = true
				cd.handshakeFailReason = 23
				return

			}
			cursor++

		case 5: // status_request
			/*
				struct {
					CertificateStatusType status_type;
					select (status_type) {
						case ocsp: OCSPStatusRequest;
					} request;
				} CertificateStatusRequest;

				enum { ocsp(1), (255) } CertificateStatusType;

				struct {
					ResponderID responder_id_list<0..2^16-1>;
					Extensions  request_extensions;
				} OCSPStatusRequest;

				opaque ResponderID<1..2^16-1>;
				opaque Extensions<0..2^16-1>;

				第一字节必须是1，然后是两字节长度，一段数据，然后又是两字节长度，一段数据
			*/
			if extensionsBs[cursor] != 1 {
				cd.DefinitelyNotTLS = true
				cd.handshakeFailReason = 24
				return

			}
			cursor++
			for i := 0; i < 2; i++ {
				l := int(extensionsBs[cursor])<<8 + int(extensionsBs[cursor+1])
				cursor += 2
				if len(extensionsBs[cursor:]) < l {
					cd.DefinitelyNotTLS = true
					cd.handshakeFailReason = 25
					return

				}
				cursor += l
			}
		case 14: //use_srtp
			/*
				https://datatracker.ietf.org/doc/html/rfc5764#section-4.1.1

				" The client MUST fill the extension_data field of the "use_srtp"
					extension with an UseSRTPData value"

				uint8 SRTPProtectionProfile[2];

				struct {
					SRTPProtectionProfiles SRTPProtectionProfiles;
					opaque srtp_mki<0..255>;
				} UseSRTPData;

				SRTPProtectionProfile SRTPProtectionProfiles<2..2^16-1>;

				前两字节长度，一段数据，1字节长度，一段数据

			*/
			l := int(extensionsBs[cursor])<<8 + int(extensionsBs[cursor+1])
			cursor += 2
			if len(extensionsBs[cursor:]) < l {
				cd.DefinitelyNotTLS = true
				cd.handshakeFailReason = 26
				return

			}
			cursor += l
			l = int(extensionsBs[cursor])
			if len(extensionsBs[cursor:]) < l {
				cd.DefinitelyNotTLS = true
				cd.handshakeFailReason = 27
				return
			}
			cursor += l

		case 15: // heartbeat
			/*
				https://datatracker.ietf.org/doc/html/rfc6520#section-2

				 enum {
					peer_allowed_to_send(1),
					peer_not_allowed_to_send(2),
					(255)
				} HeartbeatMode;

				struct {
					HeartbeatMode mode;
				} HeartbeatExtension;

				就一个字节，不是1就是2...
			*/
			b := extensionsBs[cursor]
			if b > 2 || b == 0 {
				cd.DefinitelyNotTLS = true
				cd.handshakeFailReason = 28
				return
			}
			cursor++

		case 16: //application_layer_protocol_negotiation
			//https://datatracker.ietf.org/doc/html/rfc7301#section-3.1
			/*
				 The "extension_data" field of the
				("application_layer_protocol_negotiation(16)") extension SHALL
				contain a "ProtocolNameList" value.

				opaque ProtocolName<1..2^8-1>;

				struct {
					ProtocolName protocol_name_list<2..2^16-1>
				} ProtocolNameList;



			*/

			l := int(extensionsBs[cursor])<<8 + int(extensionsBs[cursor+1])
			cursor += 2
			if len(extensionsBs[cursor:]) < l {
				cd.DefinitelyNotTLS = true
				cd.handshakeFailReason = 29
				return

			}

			if cd.ShouldSniffAlpn {

				rightEdge := cursor + l
				leftEdge := cursor
				for leftEdge < rightEdge {
					thisLen := extensionsBs[leftEdge]
					leftEdge++
					cd.SniffedAlpnList = append(cd.SniffedAlpnList, string(extensionsBs[leftEdge:leftEdge+int(thisLen)]))
					leftEdge += int(thisLen)
				}

			}
			cursor += l
		case 18: //signed_certificate_timestamp
			//https://datatracker.ietf.org/doc/html/rfc6962#section-3.3.1
			//empty "extension_data".
		case 19: //client_certificate_type
			fallthrough
		case 20: //server_certificate_type
			//https://datatracker.ietf.org/doc/html/rfc7250#section-3
			/*
				struct {
					select(ClientOrServerExtension) {
						case client:
							CertificateType client_certificate_types<1..2^8-1>;
						case server:
							CertificateType client_certificate_type;
					}
				} ClientCertTypeExtension;

				ServerCertTypeExtension 完全类似


				CertificateType 可以见

				//https://www.iana.org/assignments/tls-extensiontype-values/tls-extensiontype-values.xhtml
				中 TLS Certificate Types 部分，总之是个1字节的数据，0-3这4个值有确切的定义

				总之，可以看文档里面的握手过程，客户端和服务端都可以同时携带
				client_certificate_type 和 server_sertificate_type

				因为不仅客户端可以验证服务端，服务端也可以验证客户端，所以都是可能需要提供证书的

				然后客户端传的是一个范围，而服务端传的是一个确切值
			*/

			l := int(extensionsBs[cursor])
			if len(extensionsBs[cursor:]) < l {
				cd.DefinitelyNotTLS = true
				cd.handshakeFailReason = 30
				return
			}
			cursor += l

		case 21: //padding, 即 0x15
			//https://datatracker.ietf.org/doc/html/rfc7685#section-3
			/*
				"This memo describes a TLS extension that can be used to pad a
				ClientHello to a desired size in order to avoid implementation bugs
				caused by certain ClientHello sizes."

				The "extension_data" for the extension consists of an arbitrary
				number of zero bytes.  For example, the smallest "padding" extension
				is four bytes long and is encoded as 0x00 0x15 0x00 0x00.  A ten-byte
				extension would include six bytes of "extension_data" and would be
				encoded as:

				00 15 00 06 00 00 00 00 00 00
				|---| |---| |---------------|
					|     |           |
					|     |           \- extension_data: 6 zero bytes
					|     |
					|     \------------- 16-bit, extension_data length
					|
					\------------------- extension_type for padding extension

				The client MUST fill the padding extension completely with zero
				bytes, although the padding extension_data field may be empty.

				The server MUST NOT echo the extension.

				就是说，前两字节(即00,15后面的两字节)如果都是0，那就是 00，15，00，00，这四字节本身就占位了，算一种padding

				然后其他情况的话，前两字节(即00,15后面的两字节) 表示的是 长度n, 后面有 n 长度的 "0"; 总padding长度就是n+4

				不过我们不管总padding长度，那么实际上和其他tls 数据包的长度定义是完全类似的。

			*/

			if len(extensionsBs[cursor:]) < int(thiseLen) {
				cd.DefinitelyNotTLS = true
				cd.handshakeFailReason = 31
				return

			}
			cursor += int(thiseLen)

		/////////////////////////////////////////////////////////

		//////////////// rfc 8446 中 的内容（即 tls1.3定义 或者修订的内容）

		case 43: //supported_versions
			/*

				struct {
					select (Handshake.msg_type) {
						case client_hello:
							ProtocolVersion versions<2..254>;

						case server_hello:  and HelloRetryRequest
							ProtocolVersion selected_version;
					};
				} SupportedVersions;

				"The extension contains a list of supported
				versions in preference order, with the most preferred version first."

				这里的意思就是说，serverhello的返回值是一个固定值，而不是一长串了！所以，如果client申请了 tls1.3, 再检查一遍serverhello就可以知晓 该连接到底是1.2还是 1.3

			*/

			if isclienthello {

				wholeL := extensionsBs[cursor]
				cursor++
				if len(extensionsBs[cursor:]) < int(wholeL) {
					cd.DefinitelyNotTLS = true
					cd.handshakeFailReason = 14
					return
				}
				if wholeL%2 != 0 {
					cd.DefinitelyNotTLS = true
					cd.handshakeFailReason = 15
					return
				}
				supportedVersionsCount := int(wholeL / 2) // 每个version占两字节，且只能为 0303, 0304
				if PDD {
					log.Println("supportedVersionsCount", supportedVersionsCount, extensionsBs[cursor:cursor+int(wholeL)])
				}

				hasTls13 := false

				// 后来发现 前面会出现两个重复的未知字节 ？chrome申请时的状态
				// 发现这前面两个字节总是变的？比如第一次44，44；第二次就是137，137 等
				// 怪. 总之无法遇到未知号码就直接退出
				for i := 0; i < supportedVersionsCount; i++ {

					thisv := uint16(extensionsBs[cursor])<<8 + uint16(extensionsBs[cursor+1])

					if thisv == tls.VersionTLS13 {
						hasTls13 = true
						break
					}

					cursor++
				}

				if !hasTls13 {
					cd.CantBeTLS13 = true
				}
				//就算 申请的包含tls13，服务端也不一定支持，所以必须检验ServerHello才能确认服务端是否支持1.3
				//我们的目的就是看看到底客户端申请过tls1.3没有，现在目的达到了，可以return了

				return
			} else {
				//固定2字节;
				if thiseLen != 2 {
					cd.DefinitelyNotTLS = true
					cd.handshakeFailReason = 14
					return
				}

				thisv := uint16(extensionsBs[cursor])<<8 + uint16(extensionsBs[cursor+1])
				if thisv == tls.VersionTLS13 {

					if cd.peer.CantBeTLS13 {
						cd.DefinitelyNotTLS = true
						cd.handshakeFailReason = 15
						return
					}

					if cd.peer.handshakeVer != tls.VersionTLS12 {
						//之前的clienthello必须是 0303

						cd.DefinitelyNotTLS = true
						cd.handshakeFailReason = 16
						return
					}

					//不管别的了，直接认为握手生效。不然判断太麻烦了
					cd.helloPacketPass = true
					cd.handshakeVer = tls.VersionTLS13
					return
				} else {
					//有supported_versions字段， 里面版本号却不是 tls1.3 ,直接断定是tls1.2
					//因为tls1.3的申请只能由tls1.2的申请发送，而且1.1和1.0已经废弃了，所以我们也不考虑了
					//就算是1.1和1.0，也直接与xtls类似，直接加密转发即可，不必头大

					//如果之前客户端申请的是纯tls1.2的话，服务端也是有可能带supported_versions的，毕竟
					// rfc 没规定extension必须是客户端懂的. 只不过这样的服务端有点傻罢了...

					cd.CantBeTLS13 = true
					cd.peer.CantBeTLS13 = true
					cd.helloPacketPass = true
					return

				}

			}

		//以下都是包含两字节长度头的、我们不管的内容，直接跳过即可
		case 44: // cookie:
			/*
				struct {
					opaque cookie<1..2^16-1>;
				} Cookie;
			*/
			fallthrough
		case 13: // signature_algorithms
			fallthrough
		case 50: //signature_algorithms_cert

			/*
				struct {
					SignatureScheme supported_signature_algorithms<2..2^16-2>;
				} SignatureSchemeList;
			*/
			fallthrough
		case 47: // certificate_authorities

			/*
				struct {
					DistinguishedName authorities<3..2^16-1>;
				} CertificateAuthoritiesExtension;
			*/
			fallthrough
		case 48: // oid_filters
			/*
				struct {
					OIDFilter filters<0..2^16-1>;
				} OIDFilterExtension;

			*/
			fallthrough
		case 10: // supported_groups
			/*
				enum {

					Elliptic Curve Groups (ECDHE)
					secp256r1(0x0017), secp384r1(0x0018), secp521r1(0x0019),
					x25519(0x001D), x448(0x001E),

					Finite Field Groups (DHE)
					ffdhe2048(0x0100), ffdhe3072(0x0101), ffdhe4096(0x0102),
					ffdhe6144(0x0103), ffdhe8192(0x0104),

					Reserved Code Points
					ffdhe_private_use(0x01FC..0x01FF),
					ecdhe_private_use(0xFE00..0xFEFF),
					(0xFFFF)
				} NamedGroup;	总之就是两字节啦

				struct {
					NamedGroup named_group_list<2..2^16-1>;
				} NamedGroupList;
			*/
			fallthrough
		case 51: //key_share
			/*
				https://datatracker.ietf.org/doc/html/rfc8446#section-4.2.8

				struct {
					NamedGroup group;
					opaque key_exchange<1..2^16-1>;
				} KeyShareEntry;	前面两字节，然后两字节长度，然后一段数据

				struct {
					KeyShareEntry client_shares<0..2^16-1>;
				} KeyShareClientHello;

				struct {
					KeyShareEntry server_share;
				} KeyShareServerHello;

			*/

			if isclienthello {
				l := int(extensionsBs[cursor])<<8 + int(extensionsBs[cursor+1])
				cursor += 2
				if len(extensionsBs[cursor:]) < l {
					cd.DefinitelyNotTLS = true
					cd.handshakeFailReason = 18
					return

				}
				cursor += l
			}

		case 49: //post_handshake_auth
			/*
				struct {} PostHandshakeAuth;

				The "extension_data" field of the "post_handshake_auth" extension is
				zero length.


			*/
		case 45: //psk_key_exchange_modes
			/*
				struct {
					PskKeyExchangeMode ke_modes<1..255>;
				} PskKeyExchangeModes;
			*/
			l := int(extensionsBs[cursor])
			cursor++
			if len(extensionsBs[cursor:]) < int(l) {
				cd.DefinitelyNotTLS = true
				cd.handshakeFailReason = 19
				return

			}
			cursor += l
		case 42: //early_data
			/*
				struct {} Empty;

				struct {
					select (Handshake.msg_type) {
						case new_session_ticket:   uint32 max_early_data_size;
						case client_hello:         Empty;
						case encrypted_extensions: Empty;
					};
				} EarlyDataIndication;

				因为我们这里是 client/server hello，所以是空的, 按理说server hello不应该有这个extension？
			*/

		case 41: //pre_shared_key
			/*
				struct {
					PskIdentity identities<7..2^16-1>;
					PskBinderEntry binders<33..2^16-1>;
				} OfferedPsks;

				struct {
					select (Handshake.msg_type) {
						case client_hello: OfferedPsks;
						case server_hello: uint16 selected_identity;
					};
				} PreSharedKeyExtension;

				就是说，我们clienthello部分，是两段组成的，跳过两段.
			*/
			if isclienthello {
				for i := 0; i < 2; i++ {
					l := int(extensionsBs[cursor])<<8 + int(extensionsBs[cursor+1])
					cursor += 2
					if len(extensionsBs[cursor:]) < l {
						cd.DefinitelyNotTLS = true
						cd.handshakeFailReason = 20
						return

					}
					cursor += l
				}
			} else {
				cursor += 2
			}

		} //switch

	} //for cursor < lenE {

}
