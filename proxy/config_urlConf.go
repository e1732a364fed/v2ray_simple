package proxy

import (
	"log"
	"net/url"
)

type UrlConf struct {
	ListenUrl string `json:"listen"`
	DialUrl   string `json:"dial"`
}

// listenURL 不可为空。dialURL如果为空，会自动被设为 DirectURL
func LoadUrlConf(listenURL, dialURL string) (urlConf UrlConf, err error) {

	if dialURL == "" {
		dialURL = DirectURL
	}

	_, err = url.Parse(listenURL)
	if err != nil {
		log.Printf("listenURL given but invalid %s %s\n", listenURL, err.Error())
		return
	}

	urlConf = UrlConf{
		ListenUrl: listenURL,
	}

	_, err = url.Parse(dialURL)
	if err != nil {
		log.Printf("dialURL given but invalid %s %s\n", dialURL, err.Error())
		return
	}

	urlConf.DialUrl = dialURL

	return
}
