//go:build gui

package main

import (
	"os"

	"github.com/e1732a364fed/ui"
	"github.com/e1732a364fed/v2ray_simple/advLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
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

		if len(appVboxExtra) > 0 {
			for _, f := range appVboxExtra {
				f(vbox)
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
	group1.SetMargined(true)
	group2.SetMargined(true)

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

	allAdvs := utils.GetMapSortedKeySlice(advLayer.ProtocolsMap)

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

	curSelectedTagIdx := -1

	var update func(shouldChange bool)

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

	advCbox := ui.NewCombobox()
	form.Append("adv", advCbox, false)

	pathE := ui.NewEntry()
	form.Append("path", pathE, false)

	var muxC *ui.Checkbox

	{
		setUuid := func() {
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
		})

		tagE.OnChanged(func(e *ui.Entry) {
			if isDial {
				sc.Dial[curSelectedTagIdx].Tag = tagE.Text()
			} else {
				sc.Listen[curSelectedTagIdx].Tag = tagE.Text()
			}
			update(false)
		})

		hostE.OnChanged(func(e *ui.Entry) {
			if isDial {
				sc.Dial[curSelectedTagIdx].Host = hostE.Text()
			} else {
				sc.Listen[curSelectedTagIdx].Host = hostE.Text()
			}
		})

		ipE.OnChanged(func(e *ui.Entry) {
			if isDial {
				sc.Dial[curSelectedTagIdx].IP = ipE.Text()
			} else {
				sc.Listen[curSelectedTagIdx].IP = ipE.Text()
			}
		})

		portE.OnChanged(func(s *ui.Spinbox) {
			if isDial {
				sc.Dial[curSelectedTagIdx].Port = portE.Value()
			} else {
				sc.Listen[curSelectedTagIdx].Port = portE.Value()
			}
		})

		tlsC.OnToggled(func(c *ui.Checkbox) {
			if isDial {
				sc.Dial[curSelectedTagIdx].TLS = tlsC.Checked()
			} else {
				sc.Listen[curSelectedTagIdx].TLS = tlsC.Checked()
			}
			update(false)
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
		})

		pathE.OnChanged(func(e *ui.Entry) {
			if isDial {
				sc.Dial[curSelectedTagIdx].Path = pathE.Text()
			} else {
				sc.Listen[curSelectedTagIdx].Path = pathE.Text()
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
				}

			}
			pCbox.SetSelected(slices.Index(allProtocols, cc.Protocol))

			tagE.SetText(cc.Tag)
			hostE.SetText(cc.Host)
			ipE.SetText(cc.IP)
			portE.SetValue(cc.Port)
			uuidE.SetText(cc.UUID)
			tlsC.SetChecked(cc.TLS)
			pathE.SetText(cc.Path)

			advCbox.SetSelected(slices.Index(allAdvs, cc.AdvancedLayer))

		}

		if shouldChange {
			var shouldStart = false
			if mainM.IsRunning() {
				mainM.Stop()
				shouldStart = true
			}

			mainM.RemoveAllClient()

			mainM.LoadDialConf(sc.Dial)

			mainM.RemoveAllServer()

			mainM.LoadListenConf(sc.Listen, false)

			if shouldStart {
				mainM.Start()
			}

			mainM.PrintAllStateForHuman(os.Stdout)
		}

	}
	update(false)

	if isDial {
		applyBtn := ui.NewButton("提交修改")
		vb.Append(ui.NewHorizontalBox(), true)
		vb.Append(applyBtn, false)
		applyBtn.OnClicked(func(b *ui.Button) {
			update(true)
		})

	}
}
