package utils

import (
	"archive/zip"
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/dustin/go-humanize"
)

// TryDownloadWithProxyUrl try to download from a link with the given proxy url.
// thehttpClient is the client created, could be http.DefaultClient or a newly created one.
//
// If proxyUrl is empty, the function will call http.DefaultClient.Get, else it will create with a client with a transport with proxy set to proxyUrl. If err==nil, then thehttpClient!=nil .
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

func SimpleDownloadFile(fname, downloadLink string) (ok bool) {
	PrintStr("Downloading ")
	PrintStr(fname)
	PrintStr(" ...\n")

	_, resp, err := TryDownloadWithProxyUrl("", downloadLink)

	if err != nil {
		fmt.Printf("Download failed %s\n", err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Download got bad status: %s\n", resp.Status)
		return
	}

	out, err := os.Create(fname)
	if err != nil {
		fmt.Printf("Can Download but Can't Create File,%s \n", err.Error())
		return
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		fmt.Printf("Write downloaded to file failed: %s\n", err.Error())
		return
	}
	PrintStr("Download success!\n")

	return
}

// https://golangcode.com/download-a-file-with-progress/
type DownloadPrintCounter struct {
	Total uint64
}

func (wc *DownloadPrintCounter) Write(p []byte) (int, error) {
	n := len(p)
	wc.Total += uint64(n)
	wc.PrintProgress()
	return n, nil
}

func (wc DownloadPrintCounter) PrintProgress() {
	fmt.Printf("\r%s", strings.Repeat(" ", 35))

	fmt.Printf("\rDownloading... %s complete", humanize.Bytes(wc.Total))
}

func DownloadAndUnzip(fname, downloadLink, dst string) (ok bool) {
	PrintStr("Downloading ")
	PrintStr(fname)
	PrintStr(" ...\n")

	_, resp, err := TryDownloadWithProxyUrl("", downloadLink)

	if err != nil {
		fmt.Printf("Download failed %s\n", err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Download got bad status: %s\n", resp.Status)
		return
	}
	buf := new(bytes.Buffer)
	counter := &DownloadPrintCounter{}
	io.Copy(buf, io.TeeReader(resp.Body, counter))

	out := bytes.NewReader(buf.Bytes())

	PrintStr("\nDownload success!\n")

	reader, _ := zip.NewReader(out, int64(out.Len()))

	for _, f := range reader.File {
		filePath := filepath.Join(dst, f.Name)
		fmt.Println("unzipping file ", filePath)

		if f.FileInfo().IsDir() {
			fmt.Println("creating directory...")
			os.MkdirAll(filePath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
			fmt.Println(err)
			return
		}

		dstFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			fmt.Println(err)
			return
		}

		fileInArchive, err := f.Open()
		if err != nil {
			fmt.Println(err)
			return
		}

		if _, err := io.Copy(dstFile, fileInArchive); err != nil {
			fmt.Println(err)
			return
		}

		dstFile.Close()
		fileInArchive.Close()
	}

	ok = true
	return
}
