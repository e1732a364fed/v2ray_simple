package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"flag"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/tlsLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
)

var (
	enableApiServer     bool
	apiServerRunning    bool
	apiServerPlainHttp  bool
	apiServerKeyFile    string
	apiServerCertFile   string
	apiServerPathPrefix string
	apiServerAdminPass  string
	apiServerAddr       string
)

func init() {
	flag.BoolVar(&enableApiServer, "ea", false, "enable api server")
	flag.BoolVar(&apiServerPlainHttp, "sunsafe", false, "if given, api Server will use http instead of https")

	flag.StringVar(&apiServerPathPrefix, "spp", "/api", "api Server Path Prefix, must start with '/' ")
	flag.StringVar(&apiServerAdminPass, "sap", "", "api Server admin password, but won't be used if it's empty")
	flag.StringVar(&apiServerAddr, "sa", "127.0.0.1:48345", "api Server listen address")
	flag.StringVar(&apiServerCertFile, "scert", "", "api Server tls cert file path")
	flag.StringVar(&apiServerKeyFile, "skey", "", "api Server tls cert key path")

}

/*
curl -k https://127.0.0.1:48345/api/allstate
*/

// 非阻塞,如果运行成功则 apiServerRunning 会被设为 true
func tryRunApiServer() {

	var thepass string

	if appConf != nil {
		if ap := appConf.AdminPass; ap != "" {
			thepass = ap
		}
	} else if apiServerAdminPass != "" {
		thepass = apiServerAdminPass
	}

	apiServerRunning = true

	go runApiServer(thepass)

}

const eIllegalParameter = "illegal parameter"

// 阻塞
func runApiServer(adminUUID string) {

	var addrStr = apiServerAddr
	if apiServerPlainHttp {
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

	mux := http.NewServeMux()

	failBadRequest := func(e error, eInfo string, w http.ResponseWriter) {
		if ce := utils.CanLogWarn(eInfo); ce != nil {
			ce.Write(zap.Error(e))
		}
		w.WriteHeader(http.StatusBadRequest)
	}

	ser.addServerHandle(mux, "allstate", func(w http.ResponseWriter, r *http.Request) {
		printAllState(w)
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
			hotDeleteServer(listenIndex)
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
			hotDeleteClient(dialIndex)
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
			e := hotLoadListenUrl(listenStr, uf)
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
			e := hotLoadDialUrl(dialStr, uf)
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
			if err != nil || ind < 0 || (isDial && ind >= len(allClients)) || (!isDial && ind >= len(allServers)) {
				failBadRequest(err, eIllegalParameter, w)

				w.Write([]byte(eIllegalParameter))
				return
			}
			if isDial {
				dc := getDialConfFromCurrentState(ind)
				url := proxy.ToStandardUrl(&dc.CommonConf, dc, nil)
				w.Write([]byte(url))
			} else {
				lc := getListenConfFromCurrentState(ind)
				url := proxy.ToStandardUrl(&lc.CommonConf, nil, lc)
				w.Write([]byte(url))
			}

		}

	})

	tlsConf := &tls.Config{}

	if apiServerPlainHttp {

	} else if apiServerCertFile == "" || apiServerKeyFile == "" {
		log.Println("api server will use tls but key or cert file not provided, use random cert instead")
		tlsConf = &tls.Config{
			InsecureSkipVerify: true,
			Certificates:       tlsLayer.GenerateRandomTLSCert(), //curl -k
		}
	}

	srv := &http.Server{
		Addr:         apiServerAddr,
		Handler:      mux,
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		TLSConfig:    tlsConf,
	}

	if apiServerPlainHttp {
		srv.ListenAndServe()

	} else {
		srv.ListenAndServeTLS(apiServerCertFile, apiServerKeyFile)

	}
	apiServerRunning = false
}

type auth struct {
	expectedUsernameHash [32]byte
	expectedPasswordHash [32]byte
}

type apiServer struct {
	admin_auth auth
	nopass     bool
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
	mux.HandleFunc(apiServerPathPrefix+"/"+name, ser.basicAuth(f))
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
