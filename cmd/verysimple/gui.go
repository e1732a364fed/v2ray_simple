//go:build gui

package main

// gui界面, 所属计划为 vsc 计划，即versyimple client计划，使用图形界面. 服务端无需gui，所以我们叫client

import (
	"os"
	"syscall"

	"github.com/e1732a364fed/ui"
	_ "github.com/e1732a364fed/ui/winmanifest"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"

	_ "github.com/e1732a364fed/v2ray_simple/proxy/tun"
)

var testFunc func()

func init() {
	gui_mode = true
	runGui = func() {
		ui.Main(setupUI)
		testFunc = func() {

		}
	}
}

var mainwin *ui.Window

func makeBasicControlsPage() ui.Control {
	vbox := ui.NewVerticalBox()
	vbox.SetPadded(true)

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

		toggleCheckbox.SetChecked(defaultMachine.IsRunning())

		defaultMachine.AddToggleCallback(func(i int) {
			if mainwin == nil {
				return
			}
			ok := i == 1
			toggleCheckbox.SetChecked(ok) //SetChecked不会触发OnToggled
			if ok {
				stopBtn.Enable()
				startBtn.Disable()
			} else {
				stopBtn.Disable()
				startBtn.Enable()
			}
		})
		toggleCheckbox.OnToggled(func(c *ui.Checkbox) {
			if c.Checked() {
				defaultMachine.Start()
			} else {
				defaultMachine.Stop()

			}
		})

		stopBtn.OnClicked(func(b *ui.Button) {
			defaultMachine.Stop()
		})

		startBtn.OnClicked(func(b *ui.Button) {
			defaultMachine.Start()
		})

	}

	vbox.Append(ui.NewLabel("This is a label. Right now, labels can only span one line."), false)

	vbox.Append(ui.NewHorizontalSeparator(), false)

	group := ui.NewGroup("Entries")
	group.SetMargined(true)
	vbox.Append(group, true)

	group.SetChild(ui.NewNonWrappingMultilineEntry())

	entryForm := ui.NewForm()
	entryForm.SetPadded(true)
	group.SetChild(entryForm)

	entryForm.Append("Entry", ui.NewEntry(), false)
	entryForm.Append("Password Entry", ui.NewPasswordEntry(), false)
	entryForm.Append("Search Entry", ui.NewSearchEntry(), false)
	entryForm.Append("Multiline Entry", ui.NewMultilineEntry(), true)
	entryForm.Append("Multiline Entry No Wrap", ui.NewNonWrappingMultilineEntry(), true)

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

		ip := ui.NewProgressBar()
		ip.SetValue(-1)
		vbox.Append(ip, false)
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

	vbox.Append(ui.NewDatePicker(), false)
	vbox.Append(ui.NewTimePicker(), false)
	vbox.Append(ui.NewDateTimePicker(), false)
	vbox.Append(ui.NewFontButton(), false)
	vbox.Append(ui.NewColorButton(), false)

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

	msggrid := ui.NewGrid()
	msggrid.SetPadded(true)
	grid.Append(msggrid,
		0, 2, 2, 1,
		false, ui.AlignCenter, false, ui.AlignStart)

	button = ui.NewButton("Message Box")
	button.OnClicked(func(*ui.Button) {
		ui.MsgBox(mainwin,
			"This is a normal message box.",
			"More detailed information can be shown here.")
	})
	msggrid.Append(button,
		0, 0, 1, 1,
		false, ui.AlignFill, false, ui.AlignFill)
	button = ui.NewButton("Error Box")
	button.OnClicked(func(*ui.Button) {
		ui.MsgBoxError(mainwin,
			"This message box describes an error.",
			"More detailed information can be shown here.")
	})
	msggrid.Append(button,
		1, 0, 1, 1,
		false, ui.AlignFill, false, ui.AlignFill)

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

			} else {
				p.Signal(syscall.SIGINT)
			}
		})

		var y = ui.NewMenu("Debug")
		y.AppendItem("test").OnClicked(func(mi *ui.MenuItem, w *ui.Window) {
			if testFunc != nil {
				testFunc()
			}
		})

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
