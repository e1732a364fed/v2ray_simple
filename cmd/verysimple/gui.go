//go:build gui

package main

// gui界面, 所属计划为 vsc 计划，即versyimple client计划，使用图形界面. 服务端无需gui，所以我们叫client

import (
	"flag"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/e1732a364fed/ui"
	_ "github.com/e1732a364fed/ui/winmanifest"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
	"rsc.io/qr"
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

			const qrname = "vs_qrcode.png"

			c, err := qr.Encode("https://example.org", qr.L)
			if err != nil {
				log.Fatal(err)
			}
			pngdat := c.PNG()
			if true {
				os.WriteFile(qrname, pngdat, 0666)
			}
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

	vsHbox := ui.NewHorizontalBox()
	vsHbox.SetPadded(true)

	vsToggleGroup := ui.NewGroup("verysimple")
	vsToggleGroup.SetMargined(true)

	{
		vsVbox := ui.NewVerticalBox()
		vsVbox.SetPadded(true)

		vsToggleGroup.SetChild(vsVbox)
		vsHbox.Append(vsToggleGroup, true)
		vbox.Append(vsHbox, true)

		toggleHbox := ui.NewHorizontalBox()
		toggleHbox.SetPadded(true)
		vsVbox.Append(toggleHbox, false)

		toggleCheckbox := ui.NewCheckbox("Enable")
		stopBtn := ui.NewButton("Stop")
		startBtn := ui.NewButton("Start")

		toggleHbox.Append(toggleCheckbox, false)
		toggleHbox.Append(stopBtn, false)
		toggleHbox.Append(startBtn, false)

		toggleHbox.Append(ui.NewVerticalSeparator(), false)

		vsVbox.Append(ui.NewHorizontalSeparator(), false)

		grid := ui.NewGrid()
		grid.SetPadded(true)
		toggleHbox.Append(grid, false)

		grid.Append(ui.NewLabel("日志等级"), 0, 0, 1, 1,
			false, ui.AlignFill, false, ui.AlignFill)

		loglvl_cbox := ui.NewCombobox()

		{
			grid.Append(loglvl_cbox,
				1, 0, 1, 1,
				false, ui.AlignFill, false, ui.AlignFill)

			loglvl_cbox.Append("Debug")
			loglvl_cbox.Append("Info")
			loglvl_cbox.Append("Warn")
			loglvl_cbox.Append("Error")

			loglvl_cbox.SetSelected(utils.LogLevel)

			loglvl_cbox.OnSelected(func(c *ui.Combobox) {
				idx := loglvl_cbox.Selected()
				if idx < 0 {
					return
				}
				utils.LogLevel = idx
				utils.InitLog("log init manually")
			})

		}

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
			case <-ch:
				break
			}
		}
		toggleCheckbox.OnToggled(func(c *ui.Checkbox) {
			if c.Checked() {
				mainM.Start()
			} else {
				stopF()

			}
		})

		stopBtn.OnClicked(func(b *ui.Button) {
			stopF()
		})

		startBtn.OnClicked(func(b *ui.Button) {
			mainM.Start()
		})

		{

			fgrid := ui.NewGrid()
			fgrid.SetPadded(true)

			vsVbox.Append(fgrid, false)

			button := ui.NewButton("选择配置文件")
			confE := ui.NewEntry()
			confE.SetReadOnly(true)
			button.OnClicked(func(*ui.Button) {
				filename := ui.OpenFile(mainwin)
				if filename == "" {
					filename = "(cancelled)"
				}
				confE.SetText(filename)

				_, loadConfigErr := mainM.LoadConfig(filename, "", "")
				if loadConfigErr != nil {
					if ce := utils.CanLogErr("Gui Load Conf File Err"); ce != nil {
						ce.Write(zap.Error(loadConfigErr))
					}
				} else {
					mainM.SetupListenAndRoute()
					mainM.SetupDial()
				}

			})

			if len(configFiles) == 1 {
				confE.SetText(configFiles[0])
			}

			fgrid.Append(button,
				1, 0, 1, 1,
				false, ui.AlignFill, false, ui.AlignFill)
			fgrid.Append(confE,
				0, 0, 1, 1,
				true, ui.AlignFill, false, ui.AlignFill)

			button = ui.NewButton("保存配置文件")
			saveFE := ui.NewEntry()
			saveFE.SetReadOnly(true)
			button.OnClicked(func(*ui.Button) {
				filename := ui.SaveFile(mainwin)
				saveFE.SetText(filename)

				if filename == "" {
					saveFE.SetText("(cancelled)")
					return
				}

				vc := mainM.GetVSConfFromCurrentState()

				bs, e := utils.GetPurgedTomlBytes(vc)
				if e != nil {
					if ce := utils.CanLogErr("转换格式错误"); ce != nil {
						ce.Write(zap.Error(e))
					}

					return
				}
				filename += ".toml"
				e = os.WriteFile(filename, bs, 0666)

				if e != nil {
					if ce := utils.CanLogErr("写入文件错误"); ce != nil {
						ce.Write(zap.Error(e))
					}

					return
				}

				if ce := utils.CanLogInfo("导出成功"); ce != nil {
					ce.Write(zap.String("filename", filename))
				}

			})
			fgrid.Append(button,
				1, 1, 1, 1,
				false, ui.AlignFill, false, ui.AlignFill)
			fgrid.Append(saveFE,
				0, 1, 1, 1,
				true, ui.AlignFill, false, ui.AlignFill)

		}
	}

	vbox.Append(ui.NewHorizontalSeparator(), false)

	systemGroup := ui.NewGroup("系统代理")
	systemHbox := ui.NewHorizontalBox()
	systemHbox.Append(systemGroup, true)

	{
		systemGroup.SetMargined(true)
		systemHbox.SetPadded(true)

		vbox.Append(systemHbox, false)

		proxyForm := ui.NewForm()
		proxyForm.SetPadded(true)
		systemGroup.SetChild(proxyForm)

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
				netLayer.ToggleSystemProxy(isSocks5, addrE.Text(), portE.Text(), c.Checked())
			})

			proxyForm.Append("开关系统"+str, cb, false)
			proxyForm.Append(str+"地址", addrE, false)
			proxyForm.Append(str+"端口", portE, false)
		}

		newProxyToggle(proxyForm, true)

		newProxyToggle(proxyForm, false)
	}

	dnsGroup := ui.NewGroup("系统dns")
	systemHbox.Append(dnsGroup, true)

	{
		dnsGroup.SetMargined(true)

		dnsentryForm := ui.NewForm()
		dnsentryForm.SetPadded(true)
		dnsGroup.SetChild(dnsentryForm)

		dnsEntry := ui.NewEntry()
		if ds := netLayer.GetSystemDNS(); len(ds) > 0 {
			dnsEntry.SetText(ds[0])
		}
		dnsentryForm.Append("dns", dnsEntry, false)

		dnsConfirmBtn := ui.NewButton("提交")
		dnsConfirmBtn.OnClicked(func(b *ui.Button) {
			str := dnsEntry.Text()
			ip := net.ParseIP(str)
			if ip == nil {
				return
			}
			netLayer.SetSystemDNS(str)
		})
		dnsentryForm.Append("提交", dnsConfirmBtn, false)
	}

	entriesGroup = ui.NewGroup("Entries")
	entriesGroup.Hide()

	entriesGroup.SetMargined(true)
	vbox.Append(entriesGroup, true)

	entryForm := ui.NewForm()
	entryForm.SetPadded(true)
	entriesGroup.SetChild(entryForm)

	multilineEntry = ui.NewMultilineEntry()
	entryForm.Append("Multiline Entry", multilineEntry, true)

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

func windowClose(*ui.Window) bool {
	return true
}

func setupUI() {

	setMenu()

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
	//tab.Append("Data Choosers", makeDataChoosersPage())

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
