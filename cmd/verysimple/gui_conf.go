//go:build gui

package main

import (
	"os"
	"time"

	"github.com/e1732a364fed/ui"
	"github.com/e1732a364fed/v2ray_simple/advLayer"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
)

var appVboxExtra []func(*ui.Box)

func makeAppPage() ui.Control {
	result := ui.NewHorizontalBox()
	group := ui.NewGroup("Numbers")
	result.Append(group, true)
	group.SetMargined(true)

	{
		vbox := ui.NewVerticalBox()
		vbox.SetPadded(true)
		group.SetChild(vbox)

		{
			dtSB := ui.NewSpinbox(2, 60)
			dtSL := ui.NewSlider(2, 60)

			vbox.Append(ui.NewLabel("dial timeout (seconds)"), false)

			setDTManual := func(num float64) {
				netLayer.DialTimeout = time.Duration(num * float64(time.Second))
				if ce := utils.CanLogInfo("Manually Adjust netLayer.DialTimeout"); ce != nil {
					ce.Write(zap.Int64("seconds", int64(netLayer.DialTimeout/time.Second)))
				}
			}
			dtSB.OnChanged(func(*ui.Spinbox) {
				v := dtSB.Value()
				dtSL.SetValue(v)
				setDTManual(float64(v))
			})
			dtSL.OnChanged(func(*ui.Slider) {
				v := dtSL.Value()
				dtSB.SetValue(v)

				setDTManual(float64(v))

			})
			curV := int(netLayer.DialTimeout / time.Second)
			dtSL.SetValue(curV)
			dtSB.SetValue(curV)
			vbox.Append(dtSB, false)
			vbox.Append(dtSL, false)
			vbox.Append(ui.NewHorizontalSeparator(), false)
		}

		{
			rSB := ui.NewSpinbox(2, 60)
			rSL := ui.NewSlider(2, 60)

			vbox.Append(ui.NewLabel("read timeout (seconds)"), false)

			setRTManual := func(num float64) {
				netLayer.CommonReadTimeout = time.Duration(num * float64(time.Second))
				if ce := utils.CanLogInfo("Manually Adjust netLayer.CommonReadTimeout"); ce != nil {
					ce.Write(zap.Int64("seconds", int64(netLayer.CommonReadTimeout/time.Second)))
				}
			}
			rSB.OnChanged(func(*ui.Spinbox) {
				v := rSB.Value()
				rSL.SetValue(v)
				setRTManual(float64(v))
			})
			rSL.OnChanged(func(*ui.Slider) {
				v := rSL.Value()
				rSB.SetValue(v)

				setRTManual(float64(v))

			})
			curV := int(netLayer.CommonReadTimeout / time.Second)
			rSL.SetValue(curV)
			rSB.SetValue(curV)
			vbox.Append(rSB, false)
			vbox.Append(rSL, false)
			vbox.Append(ui.NewHorizontalSeparator(), false)
		}

		{
			uSB := ui.NewSpinbox(1, 60)
			uSL := ui.NewSlider(1, 60)

			vbox.Append(ui.NewLabel("udp timeout (minutes)"), false)

			setUTManual := func(num float64) {
				netLayer.UDP_timeout = time.Duration(num * float64(time.Minute))
				if ce := utils.CanLogInfo("Manually Adjust netLayer.UDP_timeout"); ce != nil {
					ce.Write(zap.Int64("minutes", int64(netLayer.UDP_timeout/time.Minute)))
				}
			}
			uSB.OnChanged(func(*ui.Spinbox) {
				v := uSB.Value()
				uSL.SetValue(v)
				setUTManual(float64(v))
			})
			uSL.OnChanged(func(*ui.Slider) {
				v := uSL.Value()
				uSB.SetValue(v)

				setUTManual(float64(v))

			})
			curV := int(netLayer.UDP_timeout / time.Minute)
			uSL.SetValue(curV)
			uSB.SetValue(curV)
			vbox.Append(uSB, false)
			vbox.Append(uSL, false)
			vbox.Append(ui.NewHorizontalSeparator(), false)
		}

		if len(appVboxExtra) > 0 {
			for _, f := range appVboxExtra {
				f(vbox)
				vbox.Append(ui.NewHorizontalSeparator(), false)
			}
		}
	}

	return result
}

func makeConfPage() ui.Control {

	result := ui.NewHorizontalBox()

	group1 := ui.NewGroup("Listen")
	group2 := ui.NewGroup("Dial")

	result.Append(group1, true)
	result.Append(group2, true)

	result.SetPadded(true)
	// group1.SetMargined(true)
	// group2.SetMargined(true)

	vbox1 := ui.NewVerticalBox()
	group1.SetChild(vbox1)

	vbox1.SetPadded(true)

	sc := mainM.DumpStandardConf()

	vbox2 := ui.NewVerticalBox()
	group2.SetChild(vbox2)

	addConfControls(sc, vbox1, false)
	addConfControls(sc, vbox2, true)

	// ecbox := ui.NewEditableCombobox()
	// vbox.Append(ecbox, false)

	// ecbox.Append("Editable Item 1")
	// ecbox.Append("Editable Item 2")
	// ecbox.Append("Editable Item 3")

	// rb := ui.NewRadioButtons()
	// rb.Append("Radio Button 1")
	// rb.Append("Radio Button 2")
	// rb.Append("Radio Button 3")
	// vbox.Append(rb, false)

	return result
}

func addConfControls(sc proxy.StandardConf, vb *ui.Box, isDial bool) {

	allProtocols := proxy.AllServerTypeList()
	if isDial {
		allProtocols = proxy.AllClientTypeList()
	}

	allAdvs := append([]string{""}, utils.GetMapSortedKeySlice(advLayer.ProtocolsMap)...)

	tlsTypes := []string{"tls", "utls", "shadowtls_v2"}
	if !isDial {
		utils.Splice(&tlsTypes, 1, 1)
	}

	newBtn := ui.NewButton("新增")
	rmBtn := ui.NewButton("删除")
	randUuidBtn := ui.NewButton("随机uuid")

	hbox := ui.NewHorizontalBox()
	hbox.SetPadded(true)

	hbox.Append(newBtn, false)
	hbox.Append(rmBtn, false)
	hbox.Append(randUuidBtn, false)

	vb.Append(hbox, false)

	form := ui.NewForm()
	form.SetPadded(true)
	vb.Append(form, false)

	tagCbox := ui.NewCombobox()

	form.Append("", tagCbox, false)

	pCbox := ui.NewCombobox()
	form.Append("protocol", pCbox, false)

	tagE := ui.NewEntry()
	form.Append("tag", tagE, false)

	hostE := ui.NewEntry()
	form.Append("host", hostE, false)

	ipE := ui.NewEntry()
	form.Append("ip", ipE, false)

	portE := ui.NewSpinbox(0, 65535)
	form.Append("port", portE, false)

	uuidE := ui.NewEntry()
	form.Append("uuid", uuidE, false)

	tlsC := ui.NewCheckbox("tls")
	form.Append("", tlsC, false)

	tlsCbox := ui.NewCombobox()
	form.Append("tls type", tlsCbox, false)

	tlsInsC := ui.NewCheckbox("insecure")
	form.Append("", tlsInsC, false)

	var keyE, certE *ui.Entry

	if !isDial {
		keyE = ui.NewEntry()
		form.Append("key", keyE, false)

		certE = ui.NewEntry()
		form.Append("cert", certE, false)
	}

	advCbox := ui.NewCombobox()
	form.Append("adv", advCbox, false)

	pathE := ui.NewEntry()
	form.Append("path", pathE, false)

	earlyC := ui.NewCheckbox("early")
	form.Append("", earlyC, false)

	var muxC *ui.Checkbox

	curSelectedTagIdx := -1

	var update func(shouldChange bool)

	{
		setUuid := func() {
			if curSelectedTagIdx < 0 {
				return
			}
			if isDial {
				sc.Dial[curSelectedTagIdx].UUID = uuidE.Text()
			} else {
				sc.Listen[curSelectedTagIdx].UUID = uuidE.Text()
			}
		}
		randUuidBtn.OnClicked(func(b *ui.Button) {
			uuidE.SetText(utils.GenerateUUIDStr())
			setUuid()
		})

		uuidE.OnChanged(func(e *ui.Entry) {
			setUuid()
		})

		newBtn.OnClicked(func(b *ui.Button) {
			if isDial {
				utils.Splice(&sc.Dial, curSelectedTagIdx, 0, &proxy.DialConf{})

			} else {
				utils.Splice(&sc.Listen, curSelectedTagIdx, 0, &proxy.ListenConf{})

			}
			update(false)
		})

		rmBtn.OnClicked(func(b *ui.Button) {
			if isDial {
				utils.Splice(&sc.Dial, curSelectedTagIdx, 1)

			} else {
				utils.Splice(&sc.Listen, curSelectedTagIdx, 1)

			}
			update(false)
		})

		tagCbox.OnSelected(func(c *ui.Combobox) {
			curSelectedTagIdx = tagCbox.Selected()
			update(false)
		})

		for _, p := range allProtocols {
			pCbox.Append(p)
		}

		pCbox.OnSelected(func(c *ui.Combobox) {
			if curSelectedTagIdx < 0 {
				return
			}
			idx := pCbox.Selected()

			if isDial {
				sc.Dial[curSelectedTagIdx].Protocol = allProtocols[idx]
			} else {
				sc.Listen[curSelectedTagIdx].Protocol = allProtocols[idx]
			}
			update(false)
		})

		tagE.OnChanged(func(e *ui.Entry) {
			if curSelectedTagIdx < 0 {
				return
			}
			if isDial {
				sc.Dial[curSelectedTagIdx].Tag = tagE.Text()
			} else {
				sc.Listen[curSelectedTagIdx].Tag = tagE.Text()
			}
			update(false)
		})

		hostE.OnChanged(func(e *ui.Entry) {
			if curSelectedTagIdx < 0 {
				return
			}
			if isDial {
				sc.Dial[curSelectedTagIdx].Host = hostE.Text()
			} else {
				sc.Listen[curSelectedTagIdx].Host = hostE.Text()
			}
		})

		ipE.OnChanged(func(e *ui.Entry) {
			if curSelectedTagIdx < 0 {
				return
			}
			if isDial {
				sc.Dial[curSelectedTagIdx].IP = ipE.Text()
			} else {
				sc.Listen[curSelectedTagIdx].IP = ipE.Text()
			}
		})

		portE.OnChanged(func(s *ui.Spinbox) {
			if curSelectedTagIdx < 0 {
				return
			}
			if isDial {
				sc.Dial[curSelectedTagIdx].Port = portE.Value()
			} else {
				sc.Listen[curSelectedTagIdx].Port = portE.Value()
			}
		})

		tlsC.OnToggled(func(c *ui.Checkbox) {
			if curSelectedTagIdx < 0 {
				return
			}
			if isDial {
				sc.Dial[curSelectedTagIdx].TLS = tlsC.Checked()
			} else {
				sc.Listen[curSelectedTagIdx].TLS = tlsC.Checked()
			}
			update(false)
		})

		tlsInsC.OnToggled(func(c *ui.Checkbox) {
			if curSelectedTagIdx < 0 {
				return
			}
			if isDial {
				sc.Dial[curSelectedTagIdx].Insecure = tlsInsC.Checked()
			} else {
				sc.Listen[curSelectedTagIdx].Insecure = tlsInsC.Checked()
			}
		})

		if !isDial {
			keyE.OnChanged(func(e *ui.Entry) {
				if curSelectedTagIdx < 0 {
					return
				}
				sc.Listen[curSelectedTagIdx].TLSKey = keyE.Text()
			})

			certE.OnChanged(func(e *ui.Entry) {
				if curSelectedTagIdx < 0 {
					return
				}
				sc.Listen[curSelectedTagIdx].TLSCert = certE.Text()
			})
		}

		for _, v := range tlsTypes {
			tlsCbox.Append(v)
		}

		tlsCbox.OnSelected(func(c *ui.Combobox) {
			if curSelectedTagIdx < 0 {
				return
			}
			idx := tlsCbox.Selected()

			if isDial {
				sc.Dial[curSelectedTagIdx].TlsType = tlsTypes[idx]
			} else {
				sc.Listen[curSelectedTagIdx].TlsType = tlsTypes[idx]
			}
		})

		for _, v := range allAdvs {
			advCbox.Append(v)
		}

		advCbox.OnSelected(func(c *ui.Combobox) {
			if curSelectedTagIdx < 0 {
				return
			}
			idx := advCbox.Selected()

			if isDial {
				sc.Dial[curSelectedTagIdx].AdvancedLayer = allAdvs[idx]
			} else {
				sc.Listen[curSelectedTagIdx].AdvancedLayer = allAdvs[idx]
			}

			update(false)
		})

		pathE.OnChanged(func(e *ui.Entry) {
			if curSelectedTagIdx < 0 {
				return
			}
			if isDial {
				sc.Dial[curSelectedTagIdx].Path = pathE.Text()
			} else {
				sc.Listen[curSelectedTagIdx].Path = pathE.Text()
			}
		})

		earlyC.OnToggled(func(c *ui.Checkbox) {
			if curSelectedTagIdx < 0 {
				return
			}
			if isDial {
				sc.Dial[curSelectedTagIdx].IsEarly = earlyC.Checked()
			} else {
				sc.Listen[curSelectedTagIdx].IsEarly = earlyC.Checked()
			}
		})

	}

	if isDial {
		muxC = ui.NewCheckbox("mux")
		form.Append("", muxC, false)

		muxC.OnToggled(func(c *ui.Checkbox) {
			if curSelectedTagIdx < 0 {
				return
			}

			sc.Dial[curSelectedTagIdx].Mux = muxC.Checked()
		})
	}

	update = func(shouldChange bool) {

		tagCbox.Clear()
		if isDial {

			for _, dc := range sc.Dial {
				n := dc.Tag
				if n == "" {
					n = "(no tag)"
				}
				tagCbox.Append(n)
			}
			if len(sc.Dial) > 0 && curSelectedTagIdx < 0 {
				curSelectedTagIdx = 0
			}
			if curSelectedTagIdx >= 0 && curSelectedTagIdx < len(sc.Dial) {
				tagCbox.SetSelected(curSelectedTagIdx)
			}

		} else {
			for _, lc := range sc.Listen {
				n := lc.Tag
				if n == "" {
					n = "(no tag)"
				}
				tagCbox.Append(n)
			}

			if len(sc.Listen) > 0 && curSelectedTagIdx < 0 {
				curSelectedTagIdx = 0
			}
			if curSelectedTagIdx >= 0 && curSelectedTagIdx < len(sc.Listen) {
				tagCbox.SetSelected(curSelectedTagIdx)
			}
		}

		if curSelectedTagIdx >= 0 {
			var cc proxy.CommonConf

			if isDial {
				if curSelectedTagIdx >= len(sc.Dial) {
					curSelectedTagIdx = len(sc.Dial) - 1
				}
				if curSelectedTagIdx >= 0 {
					curD := sc.Dial[curSelectedTagIdx]
					cc = curD.CommonConf

					muxC.SetChecked(curD.Mux)
				}

			} else {
				if curSelectedTagIdx >= len(sc.Listen) {
					curSelectedTagIdx = len(sc.Listen) - 1
				}

				if curSelectedTagIdx >= 0 {
					curL := sc.Listen[curSelectedTagIdx]
					cc = curL.CommonConf

					keyE.SetText(curL.TLSKey)
					certE.SetText(curL.TLSCert)

				} else {
					keyE.SetText("")
					certE.SetText("")

				}

			}
			pCbox.SetSelected(slices.Index(allProtocols, cc.Protocol))

			tagE.SetText(cc.Tag)
			hostE.SetText(cc.Host)
			ipE.SetText(cc.IP)
			portE.SetValue(cc.Port)
			uuidE.SetText(cc.UUID)
			tlsC.SetChecked(cc.TLS)
			tlsInsC.SetChecked(cc.Insecure)

			pathE.SetText(cc.Path)
			earlyC.SetChecked(cc.IsEarly)

			switch cc.Protocol {

			case "reject", "direct", "tun":
				randUuidBtn.Disable()
				hostE.Hide()
				ipE.Hide()
				portE.Hide()
				uuidE.Hide()
				tlsC.Hide()
				tlsInsC.Hide()

				pathE.Hide()
				advCbox.Hide()
				earlyC.Hide()
				if isDial {
					muxC.Hide()
				} else {
					keyE.Hide()
					certE.Hide()
				}
			case "tproxy":
				randUuidBtn.Disable()
				hostE.Hide()
				ipE.Show()
				portE.Show()
				uuidE.Hide()
				tlsC.Hide()
				tlsInsC.Hide()
				keyE.Hide()
				certE.Hide()

				pathE.Hide()
				advCbox.Hide()
				earlyC.Hide()
			default:
				randUuidBtn.Enable()
				hostE.Show()
				ipE.Show()
				portE.Show()
				uuidE.Show()
				tlsC.Show()
				tlsInsC.Show()

				pathE.Show()
				advCbox.Show()
				earlyC.Show()
				if isDial {
					muxC.Show()
				} else {
					keyE.Show()
					certE.Show()
				}
			}

			if cc.TLS {
				tlsCbox.Show()
				tlsInsC.Show()

				if !isDial {
					keyE.Show()
					certE.Show()
				}
			} else {
				tlsCbox.Hide()
				tlsInsC.Hide()

				if !isDial {
					keyE.Hide()
					certE.Hide()
				}
			}

			tlsCbox.SetSelected(slices.Index(tlsTypes, cc.TlsType))
			advCbox.SetSelected(slices.Index(allAdvs, cc.AdvancedLayer))

			if cc.AdvancedLayer != "" {
				pathE.Show()
				earlyC.Show()
			} else {
				pathE.Hide()
				earlyC.Hide()
			}

		}

		if shouldChange {
			var shouldStart = false
			if mainM.IsRunning() {
				mainM.Stop()
				shouldStart = true
			}

			if isDial {
				mainM.RemoveAllClient()
				mainM.LoadDialConf(sc.Dial)
			} else {
				mainM.RemoveAllServer()
				mainM.LoadListenConf(sc.Listen, false)
			}

			if shouldStart {
				mainM.Start()
			}

			mainM.PrintAllStateForHuman(os.Stdout, false)
		}

	}
	update(false)

	applyBtn := ui.NewButton("提交修改")
	vb.Append(ui.NewHorizontalBox(), true)
	vb.Append(applyBtn, false)
	applyBtn.OnClicked(func(b *ui.Button) {
		update(true)
	})

}
