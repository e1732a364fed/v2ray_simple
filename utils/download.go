package utils

import (
	"crypto/tls"
	"net/http"
	"net/url"
)

//if proxyUrl is empty, call http.DefaultClient.Get, or it will create with a client with a transport with proxy set to proxyUrl. If err==nil, then thehttpClient!=nil .
func TryDownloadWithProxyUrl(proxyUrl, downloadLink string) (thehttpClient *http.Client, resp *http.Response, err error) {
	thehttpClient = http.DefaultClient

	if proxyUrl == "" {
		resp, err = thehttpClient.Get(downloadLink)

	} else {
		url_proxy, e2 := url.Parse(proxyUrl)
		if e2 != nil {
			err = e2
			return
		}

		client := &http.Client{
			Transport: &http.Transport{
				Proxy:           http.ProxyURL(url_proxy),
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}

		resp, err = client.Get(downloadLink)

		thehttpClient = client
	}
	if err != nil {
		return
	}

	return
}
