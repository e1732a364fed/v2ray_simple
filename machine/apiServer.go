package machine

import (
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"flag"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/tlsLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
)

const eIllegalParameter = "illegal parameter"

/*
curl -k https://127.0.0.1:48345/api/allstate
*/

type ApiServerConf struct {
	EnableApiServer bool   `toml:"enable"`
	PlainHttp       bool   `toml:"plain"`
	KeyFile         string `toml:"key"`
	CertFile        string `toml:"cert"`
	PathPrefix      string `toml:"prefix"`
	AdminPass       string `toml:"admin_pass"`
	Addr            string `toml:"addr"`
}

// 内含默认值的 ApiServerConf
func NewApiServerConf() (ac ApiServerConf) {
	ac.SetupFlags(flag.NewFlagSet("", 10))
	return
}

// if fs == nil, flag.CommandLine will be used
func (asc *ApiServerConf) SetupFlags(fs *flag.FlagSet) {
	if fs == nil {
		fs = flag.CommandLine
	}
	fs.BoolVar(&asc.EnableApiServer, "ea", false, "enable api server")

	fs.BoolVar(&asc.PlainHttp, "sunsafe", false, "if given, api Server will use http instead of https")

	fs.StringVar(&asc.PathPrefix, "spp", "/api", "api Server Path Prefix, must start with '/' ")
	fs.StringVar(&asc.AdminPass, "sap", "", "api Server admin password, but won't be used if it's empty")
	fs.StringVar(&asc.Addr, "sa", "127.0.0.1:48345", "api Server listen address")
	fs.StringVar(&asc.CertFile, "scert", "", "api Server tls cert file path")
	fs.StringVar(&asc.KeyFile, "skey", "", "api Server tls cert key path")

}

// 若 ref 里有与默认值不同的项且字符串不为空, 将该项的值赋值给 c
func (c *ApiServerConf) SetNonDefault(ref *ApiServerConf) {
	d := NewApiServerConf()
	var emptyAc ApiServerConf

	if ref.PlainHttp != d.PlainHttp {
		c.PlainHttp = ref.PlainHttp
	}
	if ref.EnableApiServer != d.EnableApiServer {
		c.EnableApiServer = ref.EnableApiServer
	}

	if ref.Addr != d.Addr && ref.Addr != emptyAc.Addr {
		c.Addr = ref.Addr
	}
	if ref.AdminPass != d.AdminPass {
		c.AdminPass = ref.AdminPass
	}
	if ref.PathPrefix != d.PathPrefix && ref.PathPrefix != emptyAc.PathPrefix {
		c.PathPrefix = ref.PathPrefix
	}
	if ref.CertFile != d.CertFile {
		c.CertFile = ref.CertFile
	}
	if ref.KeyFile != d.KeyFile {
		c.KeyFile = ref.KeyFile
	}
}

// 非阻塞,如果运行成功则 apiServerRunning 会被设为 true
func (m *M) TryRunApiServer() {

	m.apiServerRunning = true

	go m.runApiServer()

}

// 阻塞
func (m *M) runApiServer() {

	var adminUUID string = m.AdminPass

	var addrStr = m.Addr
	if m.PlainHttp {
		if !strings.HasPrefix(addrStr, "http://") {
			addrStr = "http://" + addrStr

		}
	} else {
		if !strings.HasPrefix(addrStr, "https://") {
			addrStr = "https://" + addrStr

		}
	}

	utils.Info("Start Api Server at " + addrStr)

	ser := newApiServer("admin", adminUUID)
	ser.PathPrefix = m.PathPrefix

	mux := http.NewServeMux()

	failBadRequest := func(e error, eInfo string, w http.ResponseWriter) {
		if ce := utils.CanLogWarn(eInfo); ce != nil {
			ce.Write(zap.Error(e))
		}
		w.WriteHeader(http.StatusBadRequest)
	}

	ser.addServerHandle(mux, "allstate", func(w http.ResponseWriter, r *http.Request) {
		m.PrintAllState(w, false)
	})
	ser.addServerHandle(mux, "hotDelete", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		listenIndexStr := q.Get("listen")
		dialIndexStr := q.Get("dial")
		if listenIndexStr != "" {
			if ce := utils.CanLogInfo("api server got hot delete listen request"); ce != nil {
				ce.Write(zap.String("listenIndexStr", listenIndexStr))
			}

			listenIndex, err := strconv.Atoi(listenIndexStr)
			if err != nil {
				failBadRequest(err, eIllegalParameter, w)

				w.Write([]byte(eIllegalParameter))
				return
			}
			m.HotDeleteServer(listenIndex)
		}
		if dialIndexStr != "" {

			if ce := utils.CanLogInfo("api server got hot delete dial request"); ce != nil {
				ce.Write(zap.String("dialIndexStr", dialIndexStr))
			}

			dialIndex, err := strconv.Atoi(dialIndexStr)
			if err != nil {
				failBadRequest(err, eIllegalParameter, w)

				w.Write([]byte(eIllegalParameter))
				return
			}
			m.HotDeleteClient(dialIndex)
		}
	})

	ser.addServerHandle(mux, "hotLoadUrl", func(w http.ResponseWriter, r *http.Request) {
		if e := r.ParseForm(); e != nil {

			failBadRequest(e, "api server ParseForm failed", w)
			return

		}

		f := r.Form
		//log.Println("f", f, len(f))

		uf := proxy.UrlFormat

		listenStr := f.Get("listen")
		dialStr := f.Get("dial")
		urlFormatStr := f.Get("urlFormat")
		if urlFormatStr != "" {
			var err error
			uf, err = strconv.Atoi(urlFormatStr)
			if err != nil || uf >= proxy.Url_FormatUnknown {
				failBadRequest(utils.ErrInErr{ErrDetail: err, Data: urlFormatStr}, "api server parse urlFormat failed", w)
				return

			}
		}

		var resultStr string = "result:"
		if listenStr != "" {

			if ce := utils.CanLogInfo("api server got hot load listen request"); ce != nil {
				ce.Write(zap.String("listenUrl", listenStr))
			}
			e := m.HotLoadListenUrl(listenStr, uf)
			if e == nil {
				resultStr += "\nhot load listen Url Success for " + listenStr
			} else {
				w.WriteHeader(http.StatusInternalServerError)
				resultStr += "\nhot load listen Url Failed for " + listenStr
			}
		}
		if dialStr != "" {

			if ce := utils.CanLogInfo("api server got hot load dial request"); ce != nil {
				ce.Write(zap.String("dialUrl", dialStr))
			}
			e := m.HotLoadDialUrl(dialStr, uf)
			if e == nil {
				resultStr += "\nhot load dial Url Success for " + dialStr
			} else {
				w.WriteHeader(http.StatusInternalServerError)
				resultStr += "\nhot load dial Url Failed for " + dialStr
			}
		}
		w.Write([]byte(resultStr))
	})

	ser.addServerHandle(mux, "getDetailUrl", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		indexStr := q.Get("index")
		isDial := utils.QueryPositive(q, "isDial")
		if indexStr != "" {
			if ce := utils.CanLogInfo("api server got hot delete listen request"); ce != nil {
				ce.Write(zap.String("listenIndexStr", indexStr))
			}

			ind, err := strconv.Atoi(indexStr)
			if err != nil || ind < 0 || (isDial && ind >= len(m.allClients)) || (!isDial && ind >= len(m.allServers)) {
				failBadRequest(err, eIllegalParameter, w)

				w.Write([]byte(eIllegalParameter))
				return
			}
			if isDial {
				dc := m.dumpDialConf(ind)
				url := proxy.ToStandardUrl(&dc.CommonConf, &dc, nil)
				w.Write([]byte(url))
			} else {
				lc := m.dumpListenConf(ind)
				url := proxy.ToStandardUrl(&lc.CommonConf, nil, &lc)
				w.Write([]byte(url))
			}

		}

	})

	//保存所有配置到标准配置文件. 如果是GET, 直接将文件打印给客户, 如果是POST, 接收name参数并导出到文件
	ser.addServerHandle(mux, "dump", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		fn := q.Get("name")
		if fn == "" && r.Method == "POST" {
			if ce := utils.CanLogWarn("api server got dump request but no file name given"); ce != nil {
				ce.Write()
			}
			return
		}
		vc := m.DumpVSConf()

		bs, e := utils.GetPurgedTomlBytes(vc)
		if e != nil {
			if ce := utils.CanLogErr("api server: 转换格式错误"); ce != nil {
				ce.Write(zap.Error(e))
			}
			w.WriteHeader(500)
			w.Write([]byte("failed"))

			return
		}

		if r.Method == "GET" {
			w.Write(bs)
		} else {
			e = os.WriteFile(fn, bs, 0666)

			if e != nil {
				if ce := utils.CanLogErr("写入文件错误"); ce != nil {
					ce.Write(zap.Error(e))
				}
				w.WriteHeader(500)
				w.Write([]byte("failed"))

				return
			}

			if ce := utils.CanLogInfo("导出成功"); ce != nil {
				ce.Write(zap.String("filename", fn))
			}
			w.Write([]byte("ok"))
		}

	})

	tlsConf := &tls.Config{}

	if m.PlainHttp {

	} else if m.CertFile == "" || m.KeyFile == "" {
		log.Println("api server will use tls but key or cert file not provided, use random cert instead")
		tlsConf = &tls.Config{
			InsecureSkipVerify: true,
			Certificates:       tlsLayer.GenerateRandomTLSCert(), //curl -k
		}
	}

	srv := &http.Server{
		Addr:         m.Addr,
		Handler:      mux,
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		TLSConfig:    tlsConf,
	}

	if m.PlainHttp {
		srv.ListenAndServe()

	} else {
		srv.ListenAndServeTLS(m.CertFile, m.KeyFile)

	}
	m.apiServerRunning = false
}

type auth struct {
	expectedUsernameHash [32]byte
	expectedPasswordHash [32]byte
}

type apiServer struct {
	admin_auth auth
	nopass     bool
	PathPrefix string
}

func newApiServer(user, pass string) *apiServer {
	s := new(apiServer)

	if pass != "" {
		s.admin_auth.expectedUsernameHash = sha256.Sum256([]byte(user))
		s.admin_auth.expectedPasswordHash = sha256.Sum256([]byte(pass))

	} else {
		s.nopass = true
	}
	return s
}

func (ser *apiServer) addServerHandle(mux *http.ServeMux, name string, f func(w http.ResponseWriter, r *http.Request)) {
	mux.HandleFunc(ser.PathPrefix+"/"+name, ser.basicAuth(f))
}

func (ser *apiServer) basicAuth(realfunc http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		doFunc := func() {
			if ce := utils.CanLogInfo("api server got new request"); ce != nil {
				ce.Write(
					zap.String("method", r.Method),
					zap.String("requestURL", r.RequestURI),
				)
			}
			w.Header().Add("Access-Control-Allow-Origin", "*") //避免在网页请求本api时, 客户端遇到CSRF保护问题

			realfunc.ServeHTTP(w, r)

		}

		if ser.nopass {
			doFunc()
			return
		}

		thisun, thispass, ok := r.BasicAuth()
		if ok {
			usernameHash := sha256.Sum256([]byte(thisun))
			passwordHash := sha256.Sum256([]byte(thispass))

			usernameMatch := (subtle.ConstantTimeCompare(usernameHash[:], ser.admin_auth.expectedUsernameHash[:]) == 1)
			passwordMatch := (subtle.ConstantTimeCompare(passwordHash[:], ser.admin_auth.expectedPasswordHash[:]) == 1)

			if usernameMatch && passwordMatch {

				doFunc()
				return
			}
		}

		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
}
