//go:build gui

package main

// gui界面, 所属计划为 vsc 计划，即versyimple client计划，使用图形界面. 服务端无需gui，所以我们叫client
//tun 的支持 通过在这里引入 proxy/tun 包 实现.

import (
	"flag"
	"image"
	"image/draw"
	"log"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/e1732a364fed/ui"
	_ "github.com/e1732a364fed/ui/winmanifest"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"

	qrcode "github.com/skip2/go-qrcode"
)

var mainwin *ui.Window

var testFunc func()
var multilineEntry *ui.MultilineEntry //用于向用户提供一些随机的有用的需要复制的字符串
var entriesGroup *ui.Group            //父 of multilineEntry

type GuiPreference struct {
	HttpAddr   string `toml:"proxy_http_addr"`
	HttpPort   string `toml:"proxy_http_port"`
	Socks5Addr string `toml:"proxy_socks5_addr"`
	Socks5Port string `toml:"proxy_socks5_port"`
}

func init() {

	flag.BoolVar(&gui_mode, "g", true, "gui mode")

	runGui = func() {
		testFunc = func() {
			ce := utils.CanLogDebug("testFunc")

			if ce == nil {
				return
			}

			ce.Write()
			var strs = []string{"sfdsf", "sdfdsfdfj"}
			if multilineEntry != nil {
				entriesGroup.Show()
				multilineEntry.SetText(strings.Join(strs, "\n"))
			}

			var qr *qrcode.QRCode

			qr, err := qrcode.New("https://example.org", qrcode.Medium)

			if err != nil {
				return
			}

			const qrname = "vs_qrcode.png"
			qr.WriteFile(256, qrname)
			utils.OpenFile(qrname)

		}

		defer func() {
			utils.PrintStr("Gui Mode exited. \n")
			if ce := utils.CanLogInfo("Gui Mode exited"); ce != nil {
				ce.Write()
			}

			savePerferences()
		}()

		loadPreferences()
		setupDefaultPref()

		utils.PrintStr("Gui Mode entered. \n")
		if ce := utils.CanLogInfo("Gui Mode entered"); ce != nil {
			ce.Write()
		}

		ui.Main(setupUI)
	}

}

func setupDefaultPref() {

	if gp := currentUserPreference.Gui; gp == nil {
		gp = new(GuiPreference)
		currentUserPreference.Gui = gp

		gp.HttpAddr = "127.0.0.1"
		gp.Socks5Addr = "127.0.0.1"
		gp.HttpPort = "10800"
		gp.Socks5Port = "10800"
	}

}

func makeBasicControlsPage() ui.Control {
	vbox := ui.NewVerticalBox()
	vbox.SetPadded(true)
	vbox.Append(ui.NewLabel("开启或关闭vs代理"), false)

	hbox := ui.NewHorizontalBox()
	hbox.SetPadded(true)
	vbox.Append(hbox, false)

	toggleCheckbox := ui.NewCheckbox("Enable")
	stopBtn := ui.NewButton("Stop")
	startBtn := ui.NewButton("Start")

	hbox.Append(toggleCheckbox, false)
	hbox.Append(stopBtn, false)
	hbox.Append(startBtn, false)

	{
		var updateState = func(btn, cbx bool) {
			isR := mainM.IsRunning()

			if cbx {
				toggleCheckbox.SetChecked(isR)
			}
			if btn {
				//这里在darwin发现startBtn在被启用后，显示不出来变化，除非切换一下tab再换回来才能看出。不知何故
				if isR {
					startBtn.Disable()
					stopBtn.Enable()
				} else {
					stopBtn.Disable()
					startBtn.Enable()
				}
			}

		}

		updateState(true, true)

		mainM.AddToggleCallback(func(i int) {
			if mainwin == nil {
				return
			}
			updateState(true, true)
		})
		var stopF = func() {
			ch := make(chan struct{})
			go func() {
				mainM.Stop()
				close(ch)
			}()
			tCh := time.After(time.Second * 2)
			select {
			case <-tCh:
				log.Println("Close timeout")
				//os.Exit(-1)
			case <-ch:
				break
			}
		}
		toggleCheckbox.OnToggled(func(c *ui.Checkbox) {
			if c.Checked() {
				mainM.Start()
			} else {
				//mainM.Stop()

				stopF()

			}
		})

		stopBtn.OnClicked(func(b *ui.Button) {
			//mainM.Stop()
			stopF()
		})

		startBtn.OnClicked(func(b *ui.Button) {
			mainM.Start()
		})

	}

	vbox.Append(ui.NewHorizontalSeparator(), false)

	systemProxyGroup := ui.NewGroup("系统代理")
	systemProxyGroup.SetMargined(true)
	vbox.Append(systemProxyGroup, true)

	proxyForm := ui.NewForm()
	proxyForm.SetPadded(true)
	systemProxyGroup.SetChild(proxyForm)

	var newProxyToggle = func(form *ui.Form, isSocks5 bool) {
		gp := currentUserPreference.Gui
		var port = gp.HttpPort
		var addr = gp.HttpAddr

		str := "http"

		if isSocks5 {
			str = "socks5"
			port = gp.Socks5Port
			addr = gp.Socks5Addr
		}

		addrE := ui.NewEntry()
		addrE.SetText(addr)
		addrE.OnChanged(func(e *ui.Entry) {
			if isSocks5 {
				gp.Socks5Addr = e.Text()
			} else {
				gp.HttpAddr = e.Text()
			}
		})

		portE := ui.NewEntry()
		portE.SetText(port)
		portE.OnChanged(func(e *ui.Entry) {
			if isSocks5 {
				gp.Socks5Port = e.Text()
			} else {
				gp.HttpPort = e.Text()
			}
		})

		cb := ui.NewCheckbox("系统" + str)
		cb.OnToggled(func(c *ui.Checkbox) {
			utils.ToggleSystemProxy(isSocks5, addrE.Text(), portE.Text(), c.Checked())
		})

		proxyForm.Append("开关系统"+str, cb, false)
		proxyForm.Append(str+"地址", addrE, false)
		proxyForm.Append(str+"端口", portE, false)
	}

	newProxyToggle(proxyForm, true)

	newProxyToggle(proxyForm, false)

	entriesGroup = ui.NewGroup("Entries")
	entriesGroup.Hide()

	entriesGroup.SetMargined(true)
	vbox.Append(entriesGroup, true)

	entryForm := ui.NewForm()
	entryForm.SetPadded(true)
	entriesGroup.SetChild(entryForm)

	// entryForm.Append("Entry", ui.NewEntry(), false)
	// entryForm.Append("Password Entry", ui.NewPasswordEntry(), false)
	// entryForm.Append("Search Entry", ui.NewSearchEntry(), false)

	multilineEntry = ui.NewMultilineEntry()
	entryForm.Append("Multiline Entry", multilineEntry, true)
	// entryForm.Append("Multiline Entry No Wrap", ui.NewNonWrappingMultilineEntry(), true)

	return vbox
}

func makeConfPage() ui.Control {
	result := ui.NewHorizontalBox()
	group := ui.NewGroup("Numbers")
	group2 := ui.NewGroup("Lists")

	result.Append(group, true)
	result.Append(group2, true)

	result.SetPadded(true)
	group.SetMargined(true)
	group2.SetMargined(true)

	{
		vbox := ui.NewVerticalBox()
		vbox.SetPadded(true)
		group.SetChild(vbox)

		spinbox := ui.NewSpinbox(0, 100)
		slider := ui.NewSlider(0, 100)
		pbar := ui.NewProgressBar()
		spinbox.OnChanged(func(*ui.Spinbox) {
			slider.SetValue(spinbox.Value())
			pbar.SetValue(spinbox.Value())
		})
		slider.OnChanged(func(*ui.Slider) {
			spinbox.SetValue(slider.Value())
			pbar.SetValue(slider.Value())
		})
		vbox.Append(spinbox, false)
		vbox.Append(slider, false)
		vbox.Append(pbar, false)

		// ip := ui.NewProgressBar()
		// ip.SetValue(-1)
		// vbox.Append(ip, false)
	}

	vbox := ui.NewVerticalBox()
	group2.SetChild(vbox)

	vbox.SetPadded(true)

	hbox2 := ui.NewHorizontalBox()
	vbox.Append(hbox2, false)

	hbox2.Append(ui.NewLabel("Listen"), false)

	cbox := ui.NewCombobox()

	hbox2.Append(cbox, true)

	// cbox.Append("Combobox Item 1")
	// cbox.Append("Combobox Item 2")
	// cbox.Append("Combobox Item 3")

	hbox2 = ui.NewHorizontalBox()
	vbox.Append(hbox2, false)

	hbox2.Append(ui.NewLabel("Dial"), false)

	cbox = ui.NewCombobox()

	hbox2.Append(cbox, true)

	ecbox := ui.NewEditableCombobox()
	vbox.Append(ecbox, false)

	ecbox.Append("Editable Item 1")
	ecbox.Append("Editable Item 2")
	ecbox.Append("Editable Item 3")

	rb := ui.NewRadioButtons()
	rb.Append("Radio Button 1")
	rb.Append("Radio Button 2")
	rb.Append("Radio Button 3")
	vbox.Append(rb, false)

	return result
}

func makeDataChoosersPage() ui.Control {
	hbox := ui.NewHorizontalBox()
	hbox.SetPadded(true)

	vbox := ui.NewVerticalBox()
	vbox.SetPadded(true)
	hbox.Append(vbox, false)

	// vbox.Append(ui.NewDatePicker(), false)
	// vbox.Append(ui.NewTimePicker(), false)
	// vbox.Append(ui.NewDateTimePicker(), false)
	// vbox.Append(ui.NewFontButton(), false)
	// vbox.Append(ui.NewColorButton(), false)

	hbox.Append(ui.NewVerticalSeparator(), false)

	vbox = ui.NewVerticalBox()
	vbox.SetPadded(true)
	hbox.Append(vbox, true)

	grid := ui.NewGrid()
	grid.SetPadded(true)
	vbox.Append(grid, false)

	button := ui.NewButton("Open File")
	entry := ui.NewEntry()
	entry.SetReadOnly(true)
	button.OnClicked(func(*ui.Button) {
		filename := ui.OpenFile(mainwin)
		if filename == "" {
			filename = "(cancelled)"
		}
		entry.SetText(filename)
	})
	grid.Append(button,
		0, 0, 1, 1,
		false, ui.AlignFill, false, ui.AlignFill)
	grid.Append(entry,
		1, 0, 1, 1,
		true, ui.AlignFill, false, ui.AlignFill)

	button = ui.NewButton("Save File")
	entry2 := ui.NewEntry()
	entry2.SetReadOnly(true)
	button.OnClicked(func(*ui.Button) {
		filename := ui.SaveFile(mainwin)
		if filename == "" {
			filename = "(cancelled)"
		}
		entry2.SetText(filename)
	})
	grid.Append(button,
		0, 1, 1, 1,
		false, ui.AlignFill, false, ui.AlignFill)
	grid.Append(entry2,
		1, 1, 1, 1,
		true, ui.AlignFill, false, ui.AlignFill)

	// msggrid := ui.NewGrid()
	// msggrid.SetPadded(true)
	// grid.Append(msggrid,
	// 	0, 2, 2, 1,
	// 	false, ui.AlignCenter, false, ui.AlignStart)

	// button = ui.NewButton("Message Box")
	// button.OnClicked(func(*ui.Button) {
	// 	ui.MsgBox(mainwin,
	// 		"This is a normal message box.",
	// 		"More detailed information can be shown here.")
	// })
	// msggrid.Append(button,
	// 	0, 0, 1, 1,
	// 	false, ui.AlignFill, false, ui.AlignFill)
	// button = ui.NewButton("Error Box")
	// button.OnClicked(func(*ui.Button) {
	// 	ui.MsgBoxError(mainwin,
	// 		"This message box describes an error.",
	// 		"More detailed information can be shown here.")
	// })
	// msggrid.Append(button,
	// 	1, 0, 1, 1,
	// 	false, ui.AlignFill, false, ui.AlignFill)

	return hbox
}

func windowClose(*ui.Window) bool {
	return true
}

func setupUI() {

	var filesM = ui.NewMenu("Files")
	{
		filesM.AppendPreferencesItem()
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

		var vm = ui.NewMenu("View")
		vm.AppendItem("toggle MultilineEntry view").OnClicked(func(mi *ui.MenuItem, w *ui.Window) {
			if entriesGroup.Visible() {
				entriesGroup.Hide()
			} else {
				entriesGroup.Show()
			}
		})

		ce := utils.CanLogDebug("testFunc")

		if ce != nil {
			var y = ui.NewMenu("Debug")
			y.AppendItem("test").OnClicked(func(mi *ui.MenuItem, w *ui.Window) {
				if testFunc != nil {
					testFunc()
				}
			})

			y.AppendItem("test2").OnClicked(func(mi *ui.MenuItem, w *ui.Window) {
				qr, err := qrcode.New("https://example.org", qrcode.Medium)
				if err != nil {
					return
				}
				nw := ui.NewWindow("img", 320, 320, false)
				uiimg := ui.NewImage(320, 320)
				rect := image.Rect(0, 0, 320, 320)
				rgbaImg := image.NewRGBA(rect)
				draw.Draw(rgbaImg, rect, qr.Image(256), image.Point{}, draw.Over)
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

			y.AppendItem("test3").OnClicked(func(mi *ui.MenuItem, w *ui.Window) {
				log.Println(utils.GetSystemProxyState(true))
				log.Println(utils.GetSystemProxyState(false))
			})
		}

	}
	mainwin = ui.NewWindow("verysimple", 640, 480, true) //must create after menu; or it will panic

	{
		mainwin.OnClosing(func(*ui.Window) bool {
			ui.Quit() //只是退出gui模式，不会退出app
			mainwin = nil
			return true
		})
		ui.OnShouldQuit(func() bool {
			mainwin = nil
			return true
		})
	}

	tab := ui.NewTab()
	mainwin.SetChild(tab)
	mainwin.SetMargined(true)

	tab.Append("基础控制", makeBasicControlsPage())
	tab.Append("配置控制", makeConfPage())
	tab.Append("Data Choosers", makeDataChoosersPage())

	for i := 0; i < tab.NumPages(); i++ {
		tab.SetMargined(i, true)
	}
	mainwin.Show()

}

type imgTableH struct {
	img *ui.Image
}

func newImgTableHandler() *imgTableH {
	m := new(imgTableH)
	return m
}

func (mh *imgTableH) ColumnTypes(m *ui.TableModel) []ui.TableValue {
	return []ui.TableValue{

		ui.TableImage{}, // column 1 image

	}
}

func (mh *imgTableH) NumRows(m *ui.TableModel) int {
	return 2
}

func (mh *imgTableH) CellValue(m *ui.TableModel, row, column int) ui.TableValue {

	return ui.TableImage{I: mh.img}

}

func (mh *imgTableH) SetCellValue(m *ui.TableModel, row, column int, value ui.TableValue) {

}
