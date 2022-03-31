package main

import (
	"io"
	"log"
	"net"
	"runtime"
	"strconv"

	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/proxy"
	"github.com/hahahrfool/v2ray_simple/proxy/vless"
	"github.com/hahahrfool/v2ray_simple/tlsLayer"
	"github.com/hahahrfool/v2ray_simple/utils"
)

const tlslazy_willuseSystemCall = runtime.GOOS == "linux" || runtime.GOOS == "darwin"

//grpc 这种多路复用的链接是绝对无法开启 lazy的, ws 理论上也只有服务端发向客户端的链接 内嵌tls时可以lazy，但暂不考虑
func canLazyEncryptServer(inServer proxy.Server) bool {

	return inServer.IsUseTLS() && canNetwork_tlsLazy(inServer.Network()) && inServer.AdvancedLayer() == ""
}

func canLazyEncryptClient(outClient proxy.Client) bool {

	return outClient.IsUseTLS() && canNetwork_tlsLazy(outClient.Network()) && outClient.AdvancedLayer() == ""
}

func canNetwork_tlsLazy(nw string) bool {
	switch nw {
	case "", "tcp", "tcp4", "tcp6", "unix":
		return true
	}
	return false
}

// tryTlsLazyRawCopy 尝试能否直接对拷，对拷 直接使用 原始 TCPConn，也就是裸奔转发
//  如果在linux上，则和 xtls的splice 含义相同. 在其他系统时，与xtls-direct含义相同。
// 我们内部先 使用 DetectConn进行过滤分析，然后再判断进化为splice 或者退化为普通拷贝
// 第一个参数仅用于 tls_lazy_secure
func tryTlsLazyRawCopy(useSecureMethod bool, proxy_client proxy.UserClient, proxy_server proxy.UserServer, targetAddr netLayer.Addr, wrc, wlc io.ReadWriteCloser, localConn net.Conn, isclient bool, theRecorder *tlsLayer.Recorder) {
	if utils.CanLogDebug() {
		log.Printf("trying tls lazy copy\n")
	}

	//如果用了 lazy_encrypt， 则不直接利用Copy，因为有两个阶段：判断阶段和直连阶段
	// 在判断阶段，因为还没确定是否是 tls，所以是要继续用tls加密的，
	// 而直连阶段，只要能让 Copy使用 net.TCPConn的 ReadFrom, 就不用管了， golang最终就会使用splice
	// 之所以可以对拷直连，是因为无论是 socks5 还是vless，只是在最开始的部分 加了目标头，后面的所有tcp连接都是直接传输的数据，就是说，一开始握手什么的是不能直接对拷的，等到后期就可以了
	// 而且之所以能对拷，还有个原因就是，远程服务器 与 客户端 总是源源不断地 为 我们的 原始 TCP 连接 提供数据，我们只是一个中间商而已，左手倒右手

	// 如果是客户端，则 从 wlc 读取，写入 wrc ，这种情况是 Write, 然后对于 DetectConn 来说是 Read，即 从DetectConn读取，然后 写入到远程连接
	// 如果是服务端，则 从 wrc 读取，写入 wlc， 这种情况是 Write
	//
	// 总之判断 Write 的对象，是考虑 客户端和服务端之间的数据传输，不考虑 远程真实服务器

	wlcdc := wlc.(*tlsLayer.SniffConn)
	wlccc_raw := wlcdc.RawConn

	if isclient {
		sc := []byte(proxy_client.GetUser().GetIdentityBytes())
		wlcdc.R.SpecialCommandBytes = sc
		wlcdc.W.SpecialCommandBytes = sc
	} else {
		wlcdc.R.UH = proxy_server
	}

	var rawWRC *net.TCPConn

	if !useSecureMethod {

		//wrc 有两种情况，如果客户端那就是tls，服务端那就是direct。我们不讨论服务端 处于中间层的情况

		if isclient {
			// 不过实际上客户端 wrc 是 vless的 UserConn， 而UserConn的底层连接才是TLS
			// 很明显，目前我们只支持vless所以才可这么操作，以后再说。

			wrcVless := wrc.(*vless.UserConn)
			tlsConn := wrcVless.Conn.(*tlsLayer.Conn)
			rawWRC = tlsConn.GetRaw(tls_lazy_encrypt)

			//不过仔细思考，我们根本不需要这么繁琐地获取啊？！因为我们的 原始连接我们本来就是有的！
			//rawWRC = localConn.(*net.TCPConn) //然而我实测，竟然传输会得到错误的结果，怎么回事

		} else {
			rawWRC = wrc.(*net.TCPConn) //因为是direct
		}

		if rawWRC == nil {
			if tlsLayer.PDD {
				log.Printf("splice fail reason 0\n")

			}

			if tls_lazy_encrypt {
				theRecorder.StopRecord()
				theRecorder.ReleaseBuffers()
			}

			//退化回原始状态
			go io.Copy(wrc, wlc)
			io.Copy(wlc, wrc)
			return
		}
	} else {
		rawWRC = wrc.(*net.TCPConn) //useSecureMethod的一定是客户端，此时就是直接给出原始连接
	}

	waitWRC_CreateChan := make(chan int)

	go func(wrcPtr *io.ReadWriteCloser) {
		//从 wlccc 读取，向 wrc 写入
		// 此时如果ReadFrom，那就是 wrc.ReadFrom(wlccc)
		//wrc 要实现 ReaderFrom才行, 或者把最底层TCPConn暴露，然后 wlccc 也要把最底层 TCPConn暴露出来
		// 这里就直接采取底层方式

		p := utils.GetPacket()
		isgood := false
		isbad := false

		checkCount := 0

		for {
			if isgood || isbad {
				break
			}
			n, err := wlcdc.Read(p)
			if err != nil {
				break
			}

			checkCount++

			if useSecureMethod && checkCount == 1 {
				//此时还未dial，需要进行dial; 仅限客户端

				if tlsLayer.PDD {
					log.Printf(" 才开始Dial 服务端\n")

				}

				theRecorder = tlsLayer.NewRecorder()
				teeConn := tlsLayer.NewTeeConn(rawWRC, theRecorder)

				tlsConn, err := proxy_client.GetTLS_Client().Handshake(teeConn)
				if err != nil {
					if utils.CanLogErr() {
						log.Printf("failed in handshake outClient tls , Reason: %s\n", err)

					}
					return
				}

				wrc, err = proxy_client.Handshake(tlsConn, targetAddr)
				if err != nil {
					if utils.CanLogErr() {
						log.Printf("failed in handshake to %s , Reason: %s\n", targetAddr.String(), err)
					}
					return
				}

				*wrcPtr = wrc

				waitWRC_CreateChan <- 1

			} else {
				if tlsLayer.PDD {
					log.Printf("第 %s 次测试", strconv.Itoa(checkCount))
				}
			}

			//wrc.Write(p[:n])
			//在判断 “是TLS” 的瞬间，它会舍弃写入数据，而把写入的主动权交回我们，我们发送特殊命令后，通过直连写入数据
			if wlcdc.R.IsTls && wlcdc.RawConn != nil {
				isgood = true

				if isclient {

					// 若是client，因为client是在Read时判断的 IsTLS，所以特殊指令实际上是要在这里发送

					if tlsLayer.PDD {
						log.Printf("R 准备发送特殊命令, 以及保存的TLS内容,%d\n", len(p[:n]))
					}

					wrc.Write(wlcdc.R.SpecialCommandBytes)

					//然后还要发送第一段FreeData

					rawWRC.Write(p[:n])

				} else {

					//如果是 server, 则 此时   已经收到了解密后的 特殊指令
					// 我们要从 theRecorder 中最后一个Buf里找 原始数据

					//theRecorder.DigestAll()

					//这个瞬间，R是存放了 SpecialCommand了（即uuid），然而W还是没有的 ，
					// 所以我们要先给W的SpecialCommand 赋值

					wlcdc.W.SpecialCommandBytes = wlcdc.R.SpecialCommandBytes

					rawBuf := theRecorder.GetLast()
					bs := rawBuf.Bytes()

					/*
						if tlsLayer.PDD {
							_, record_count := tlsLayer.GetLastTlsRecordTailIndex(bs)
							if record_count < 2 { // 应该是0-rtt的情况

								log.Println("检测到0-rtt"")

							}
							log.Println("R Recorder 中记录了", record_count, "条记录")
						}
					*/

					nextI := tlsLayer.GetTlsRecordNextIndex(bs)

					//有可能只存有一个record，然后 supposedLen非常长，此时 nextI是大于整个bs长度的
					//正常来说这是不应该发生的，但是实际测速时发生了！会导致服务端闪退，
					// 就是说在客户端上传大流量时，可能导致服务端出问题
					//
					//仔细思考，如果在客户端发送特殊指令的同时，tls的Conn仍然在继续写入的话，那么就有可能出现这种情况，
					// 也就是说，是多线程问题；但是还是不对，如果tls正在写入，那么我们则还没达到写特殊指令的代码
					//只能说，写入的顺序完全是正确的，但是收到的数据似乎有错误发生
					//
					// 还是说，特殊指令实际上被分成了两个record发送？这么短的指令也能吗？
					//还有一种可能？就是在写入“特殊指令”的瞬间，需要发送一些alert？然后在特殊指令的前后一起发送了？
					//仅在 ubuntu中测试发生过，macos中 测试从未发生过

					//总之，实际测试这个 nextI 似乎特别大，然后bs也很大。bs大倒是正常，因为是测速
					//
					// 一种情况是，特殊指令粘在上一次tls包后面被一起发送。那么此时lastbuffer应该完全是新自由数据
					// 上一次的tls包应该是最后一个握手包。但是问题是，client必须要收到服务端的握手回应才能继续发包
					// 所以还是不应该发生。
					//  除非，使用了某种方式在握手的同时也传递数据，等等，tls1.3的0-rtt就是如此啊！
					//
					// 而且，上传正好属于握手的同时上传数据的情况。
					// 而服务端是无法进行tls1.3的0-rtt的，因为理论上 tls1.3的 0-rtt只能由客户端发起。
					// 所以才会出现下载时毫无问题，上传时出bug的情况
					//
					//如果是0-rtt，我们的Recorder应该根本没有记录到我们的特殊指令包，因为它是从第二个包开始记录的啊！
					// 所以我们从Recorder获取到的包是不含“特殊指令”包的，所以获取到的整个数据全是我们想要的

					if len(bs) < nextI {
						// 应该是 tls1.3 0-rtt的情况

						rawWRC.Write(bs)

					} else {
						nextFreeData := bs[nextI:]

						if tlsLayer.PDD {
							log.Printf("R 从Recorder 提取 真实TLS内容, %d\n", len(nextFreeData))

						}
						rawWRC.Write(nextFreeData)

					}

					theRecorder.StopRecord()
					theRecorder.ReleaseBuffers()

				}
			} else {

				if tlsLayer.PDD {
					log.Printf("pass write\n")

				}
				wrc.Write(p[:n])
				if wlcdc.R.DefinitelyNotTLS {
					isbad = true
				}
			}
		}
		utils.PutPacket(p)

		if isbad {
			//直接退化成普通Copy

			if tlsLayer.PDD {
				log.Printf("SpliceRead R方向 退化…… %d\n", wlcdc.R.GetFailReason())
			}

			netLayer.TryCopy(wrc, wlc)

			wrc.Close()
			wlc.Close()
			return
		}

		if isgood {
			if tlslazy_willuseSystemCall {
				runtime.Gosched() //详情请阅读我的 xray_splice- 文章，了解为什么这个调用是必要的
			}

			if tlsLayer.PDD {
				log.Printf("成功SpliceRead R方向\n")
				num, e1 := rawWRC.ReadFrom(wlccc_raw)
				log.Printf("SpliceRead R方向 传完，%s , 长度: %d\n", e1, num)
			} else {
				rawWRC.ReadFrom(wlccc_raw)
			}

			wrc.Close()
			wlc.Close()
		}
	}(&wrc)

	isgood2 := false
	isbad2 := false

	p := utils.GetPacket()

	count := 0

	//从 wrc  读取，向 wlccc 写入
	for {
		if isgood2 || isbad2 {
			break
		}

		count++

		if useSecureMethod && count == 1 {
			<-waitWRC_CreateChan
		}
		if tlsLayer.PDD {
			log.Printf("准备从wrc读\n")
		}
		n, err := wrc.Read(p)
		if err != nil {
			break
		}

		if tlsLayer.PDD {
			log.Printf("从wrc读到数据，%d 准备写入wlcdc", n)
		}

		wn, _ := wlcdc.Write(p[:n])

		if tlsLayer.PDD {
			log.Printf("写入wlcdc完成 %d\n", wn)
		}

		if wlcdc.W.IsTls && wlcdc.RawConn != nil {

			if isclient {
				//读到了服务端 发来的 特殊指令

				rawBuf := theRecorder.GetLast()
				bs := rawBuf.Bytes()

				nextI := tlsLayer.GetTlsRecordNextIndex(bs)
				if nextI > len(bs) { //理论上有可能，但是又不应该，收到的buf不应该那么短，应该至少包含一个有效的整个tls record，因为此时理论上已经收到了服务端的 特殊指令，它是单独包在一个 tls record 里的

					//不像上面类似的一段，这个例外从来没有被触发过，也就是说，下载方向是毫无问题的
					//
					//这是因为, 触发上一段类似代码的原因是tls 0-rtt，而 tls 0-rtt 总是由 客户端发起的
					log.Println("有问题， nextI > len(bs)", nextI, len(bs))
					//os.Exit(-1) //这里就暂时不要退出程序了，毕竟理论上有可能由一些黑客来触发这里。
					localConn.Close()
					rawWRC.Close()
					return
				}

				if nextI < len(bs) {
					//有额外的包
					nextFreeData := bs[nextI:]

					wlccc_raw.Write(nextFreeData)
				}

				theRecorder.StopRecord()
				theRecorder.ReleaseBuffers()

			} else {

				//此时已经写入了 特殊指令，需要再发送 freedata

				wlccc_raw.Write(p[:n])
			}

			isgood2 = true

		} else if wlcdc.W.DefinitelyNotTLS {
			isbad2 = true
		}
	}
	utils.PutPacket(p)

	if isbad2 {

		if tlsLayer.PDD {
			log.Println("SpliceRead W方向 退化……", wlcdc.W.GetFailReason())
		}
		//就算不用splice, 一样可以用readv来在读那一端增强性能
		netLayer.TryCopy(wlc, wrc)

		wrc.Close()
		wlc.Close()
		return
	}

	if isgood2 {
		if tlsLayer.PDD {
			log.Printf("成功SpliceRead W方向,准备 直连对拷\n")
		}

		if tlslazy_willuseSystemCall {
			runtime.Gosched() //详情请阅读我的 xray_splice- 文章，了解为什么这个调用是必要的

		}
		if tlsLayer.PDD {

			num, e2 := wlccc_raw.ReadFrom(rawWRC) //看起来是ReadFrom，实际上是向 wlccc_raw进行Write，即箭头向左
			log.Printf("SpliceRead W方向 传完，%s , 长度: %d\n", e2, num)
		} else {
			wlccc_raw.ReadFrom(rawWRC)
		}

		wrc.Close()
		wlc.Close()
	}

}
