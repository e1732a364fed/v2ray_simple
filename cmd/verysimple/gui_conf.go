//go:build gui

package main

import (
	"os"

	"github.com/e1732a364fed/ui"
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

		// ip := ui.NewProgressBar()
		// ip.SetValue(-1)
		// vbox.Append(ip, false)

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

	hbox2 := ui.NewHorizontalBox()
	vbox1.Append(hbox2, false)

	hbox2.Append(ui.NewLabel("Listen"), false)

	listenCbox := ui.NewCombobox()

	hbox2.Append(listenCbox, true)

	sc := mainM.DumpStandardConf()

	for i, lc := range sc.Listen {
		n := lc.Tag
		if n == "" {
			n = "(no tag)"
		}
		listenCbox.Append(n)

		listenCbox.SetSelected(i)
	}

	vbox2 := ui.NewVerticalBox()
	group2.SetChild(vbox2)

	hbox2 = ui.NewHorizontalBox()
	vbox2.Append(hbox2, false)

	hbox2.Append(ui.NewLabel("Dial"), false)

	dialCbox := ui.NewCombobox()

	hbox2.Append(dialCbox, true)

	curSelectedDial := -1

	var update func(shouldChange bool)

	for i, dc := range sc.Dial {
		n := dc.Tag
		if n == "" {
			n = "(no tag)"
		}
		dialCbox.Append(n)
		curSelectedDial = i
		dialCbox.SetSelected(curSelectedDial)
	}
	dialCbox.OnSelected(func(c *ui.Combobox) {
		curSelectedDial = dialCbox.Selected()
		update(false)
	})

	muxC := ui.NewCheckbox("mux")
	if curSelectedDial >= 0 {
		muxC.SetChecked(sc.Dial[curSelectedDial].Mux)
	}
	vbox2.Append(muxC, false)
	muxC.OnToggled(func(c *ui.Checkbox) {
		sc.Dial[curSelectedDial].Mux = muxC.Checked()
		update(true)
	})

	update = func(shouldChange bool) {
		muxC.SetChecked(sc.Dial[curSelectedDial].Mux)

		if shouldChange {
			var shouldStart = false
			if mainM.IsRunning() {
				mainM.Stop()
				shouldStart = true
			}

			mainM.RemoveAllClient()

			mainM.LoadDialConf(sc.Dial)

			if shouldStart {
				mainM.Start()
			}

			mainM.PrintAllStateForHuman(os.Stdout)
		}

	}

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
