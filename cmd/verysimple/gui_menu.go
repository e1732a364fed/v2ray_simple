//go:build gui

package main

import (
	"log"
	"net"
	"os"
	"syscall"

	"github.com/e1732a364fed/ui"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
)

func setMenu() {

	var filesM = ui.NewMenu("Files")
	{
		filesM.AppendAboutItem().OnClicked(func(mi *ui.MenuItem, w *ui.Window) {
			ui.MsgBox(mainwin,
				"verysimple, a very simple proxy",
				versionStr()+"\n\n"+weblink)
		})
		filesM.AppendQuitItem()
		openUrlFunc := func(url string) func(mi *ui.MenuItem, w *ui.Window) {
			return func(mi *ui.MenuItem, w *ui.Window) {
				if e := utils.Openbrowser(url); e != nil {
					if ce := utils.CanLogErr("open github link failed"); ce != nil {
						ce.Write(zap.Error(e))
					}
				}
			}
		}
		filesM.AppendItem("Open github").OnClicked(openUrlFunc(weblink))
		filesM.AppendItem("Check github releases").OnClicked(openUrlFunc(weblink + "releases"))
		filesM.AppendItem("Quit App").OnClicked(func(mi *ui.MenuItem, w *ui.Window) {
			//syscall.Kill(syscall.Getpid(), syscall.SIGINT) //退出app ,syscall.Kill 在windows上不存在

			if p, err := os.FindProcess(os.Getpid()); err != nil {
				if ce := utils.CanLogWarn("Failed call os.FindProcess"); ce != nil {
					ce.Write(zap.Error(err))
				}
			} else {
				p.Signal(syscall.SIGINT) //这个方法在windows上不好使
			}
		})
		filesM.AppendSeparator()

		filesM.AppendItem("Download Geoip( GeoLite2-Country.mmdb)").OnClicked(func(mi *ui.MenuItem, w *ui.Window) {
			tryDownloadMMDB()
		})

		filesM.AppendItem("Download Geosite folder(v2fly/domain-list-community)").OnClicked(func(mi *ui.MenuItem, w *ui.Window) {
			tryDownloadGeositeSource()
		})

	}

	var viewM = ui.NewMenu("View")
	viewM.AppendItem("toggle MultilineEntry view").OnClicked(func(mi *ui.MenuItem, w *ui.Window) {
		if entriesGroup.Visible() {
			entriesGroup.Hide()
		} else {
			entriesGroup.Show()
		}
	})

	var sysM = ui.NewMenu("System")
	sysM.AppendItem("获取网卡信息").OnClicked(func(mi *ui.MenuItem, w *ui.Window) {
		ifs, err := net.Interfaces()
		if err != nil {
			if ce := utils.CanLogErr("net.Interfaces() err"); ce != nil {
				ce.Write(zap.Error(err))
			}
			return
		}
		for i, v := range ifs {
			log.Println(i, v.Name, v)
		}
	})

	debugMenu()
}

func debugMenu() {
	ce := utils.CanLogDebug("testFunc")

	if ce == nil {
		return
	}

	var debugM = ui.NewMenu("Debug")
	debugM.AppendItem("test").OnClicked(func(mi *ui.MenuItem, w *ui.Window) {
		if testFunc != nil {
			testFunc()
		}
	})

	/*
		debugM.AppendItem("test2").OnClicked(func(mi *ui.MenuItem, w *ui.Window) {
			c, err := qr.Encode("https://example.org", qr.L)
			if err != nil {
				return
			}

			nw := ui.NewWindow("img", 320, 320, false)
			uiimg := ui.NewImage(320, 320)
			rect := image.Rect(0, 0, 320, 320)
			rgbaImg := image.NewRGBA(rect)
			draw.Draw(rgbaImg, rect, c.Image(), image.Point{}, draw.Over)
			uiimg.Append(rgbaImg)

			mh := newImgTableHandler()
			mh.img = uiimg
			model := ui.NewTableModel(mh)

			table := ui.NewTable(&ui.TableParams{
				Model:                         model,
				RowBackgroundColorModelColumn: 3,
			})
			table.OnRowClicked(func(t *ui.Table, i int) {
				log.Println("tc", i)
			})
			table.OnRowDoubleClicked(func(t *ui.Table, i int) {
				log.Println("tc", i)
			})
			table.OnHeaderClicked(func(t *ui.Table, i int) {
				log.Println("tc h", i)
			})
			//table.SetHeaderVisible(false)

			table.AppendImageColumn("QRCode", 0)
			table.AppendImageColumn("QRCode", 1)
			table.SetHeaderSortIndicator(0, 1)
			log.Println("tcsi", table.HeaderSortIndicator(0))
			table.SetColumnWidth(0, 2)
			nw.SetChild(table)
			nw.SetMargined(true)
			nw.OnClosing(func(w *ui.Window) bool { return true })
			nw.Show()
		})
	*/

	debugM.AppendItem("test3").OnClicked(func(mi *ui.MenuItem, w *ui.Window) {
		log.Println(netLayer.GetSystemProxyState(true))
		log.Println(netLayer.GetSystemProxyState(false))
	})
}
