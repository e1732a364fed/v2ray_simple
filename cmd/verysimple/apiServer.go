package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"flag"
	"net/http"
	"strconv"
	"time"

	"github.com/e1732a364fed/v2ray_simple/tlsLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
)

var (
	enableApiServer     bool
	apiServerRunning    bool
	apiServerPathPrefix string
	apiServerAdminPass  string
	apiServerAddr       string
)

func init() {
	flag.BoolVar(&enableApiServer, "ea", false, "enable api server")
	flag.StringVar(&apiServerPathPrefix, "spp", "/api", "api Server Path Prefix, must start with '/' ")
	flag.StringVar(&apiServerAdminPass, "sap", "", "api Server admin password, but won't be used if it's empty")
	flag.StringVar(&apiServerAddr, "sa", "127.0.0.1:48345", "api Server listen address")

}

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

// 阻塞
func runApiServer(adminUUID string) {

	utils.Info("Start Api Server at " + apiServerAddr)

	ser := newApiServer("admin", adminUUID)

	mux := http.NewServeMux()

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
				w.Write([]byte("illegal parameter"))
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
				w.Write([]byte("illegal parameter"))
				return
			}
			hotDeleteClient(dialIndex)
		}
	})

	ser.addServerHandle(mux, "hotLoadUrl", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		listenStr := q.Get("listen")
		dialStr := q.Get("dial")

		f := r.Form

		if listenStr == "" {
			listenStr = f.Get("listen")
		}
		if dialStr == "" {
			dialStr = f.Get("dial")
		}

		var resultStr string = "result:"
		if listenStr != "" {
			if ce := utils.CanLogInfo("api server got hot load listen request"); ce != nil {
				ce.Write(zap.String("listenUrl", listenStr))
			}
			e := hotLoadListenUrl(listenStr)
			if e == nil {
				resultStr += "\nhot load listen Url Success for " + listenStr
			} else {
				resultStr += "\nhot load listen Url Failed for " + listenStr
			}
		}
		if dialStr != "" {
			if ce := utils.CanLogInfo("api server got hot load dial request"); ce != nil {
				ce.Write(zap.String("dialUrl", dialStr))
			}
			e := hotLoadDialUrl(dialStr)
			if e == nil {
				resultStr += "\nhot load dial Url Success for " + dialStr
			} else {
				resultStr += "\nhot load dial Url Failed for " + dialStr
			}
		}
		w.Write([]byte(resultStr))
	})

	srv := &http.Server{
		Addr:         apiServerAddr,
		Handler:      mux,
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		TLSConfig: &tls.Config{
			InsecureSkipVerify: true,
			Certificates:       tlsLayer.GenerateRandomTLSCert(),
		},
	}

	srv.ListenAndServeTLS("", "")
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

		if ser.nopass {
			realfunc.ServeHTTP(w, r)
			return
		}

		thisun, thispass, ok := r.BasicAuth()
		if ok {
			usernameHash := sha256.Sum256([]byte(thisun))
			passwordHash := sha256.Sum256([]byte(thispass))

			usernameMatch := (subtle.ConstantTimeCompare(usernameHash[:], ser.admin_auth.expectedUsernameHash[:]) == 1)
			passwordMatch := (subtle.ConstantTimeCompare(passwordHash[:], ser.admin_auth.expectedPasswordHash[:]) == 1)

			if usernameMatch && passwordMatch {
				realfunc.ServeHTTP(w, r)
				return
			}
		}

		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
}
