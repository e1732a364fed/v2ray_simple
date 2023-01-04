//go:build gui

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/e1732a364fed/ui"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
)

type logStruct struct {
	L string `json:"L"`
	T string `json:"T"`
	M string `json:"M"`
}

func makeLogPage() ui.Control {

	vb := ui.NewVerticalBox()

	newBtn := ui.NewButton("打开")

	loglvl_cbox := ui.NewCombobox()

	hb := ui.NewHorizontalBox()
	hb.SetPadded(true)

	hb.Append(newBtn, false)
	hb.Append(loglvl_cbox, false)

	vb.Append(hb, false)

	oldTable := ui.NewTable(&ui.TableParams{Model: ui.NewTableModel(newLogTableHandler())})

	vb.Append(oldTable, false)

	curShowLvl := 0

	var reloadTable = func(bs []byte) {
		vb.Delete(1)
		oldTable.Destroy()

		h := newLogTableHandler()

		str := string(bs)
		lines := strings.Split(str, "\n")

		for _, l := range lines {

			var s1 logStruct
			json.Unmarshal([]byte(l), &s1)

			if curShowLvl != 0 {
				if strings.ToLower(strings.TrimSpace(s1.L)) != utils.LogLevelStr(curShowLvl-1) {
					continue
				}
			}

			var m1 map[string]any
			json.Unmarshal([]byte(l), &m1)

			delete(m1, "L")
			delete(m1, "T")
			delete(m1, "M")

			var item logItem
			item.s = s1
			item.m = m1

			h.items = append(h.items, item)
		}

		model := ui.NewTableModel(h)

		table := ui.NewTable(&ui.TableParams{
			Model:                         model,
			RowBackgroundColorModelColumn: 3,
		})

		table.AppendTextColumn("idx", 0, 0, nil)

		tp1 := ui.TableTextColumnOptionalParams{
			ColorModelColumn: 15,
		}
		table.AppendTextColumn("Level", 1, 1, &tp1)
		table.AppendTextColumn("Time", 2, 2, nil)
		table.AppendTextColumn("Message", 3, 3, nil)
		table.AppendTextColumn("Other", 4, 4, nil)

		table.SetColumnWidth(0, 20)

		vb.Append(table, false)
		oldTable = table
	}

	var curBs []byte

	newBtn.OnClicked(func(b *ui.Button) {
		filename := ui.OpenFile(mainwin)
		notok := false
		if filename == "" {
			notok = true
		} else if !utils.FileExist(filename) {
			notok = true
		}
		if notok {
			return
		}
		bs, err := os.ReadFile(filename)
		if err != nil {
			if ce := utils.CanLogErr("Failed gui open log file "); ce != nil {
				ce.Write(zap.Error(err))
			}
			return
		}

		curBs = bs
		reloadTable(bs)

	})

	{
		loglvl_cbox.Append("")
		ll := utils.LogLevel5CharList()

		for _, l := range ll {
			loglvl_cbox.Append(l)
		}

		loglvl_cbox.OnSelected(func(c *ui.Combobox) {
			idx := loglvl_cbox.Selected()
			if idx < 0 {
				return
			}
			curShowLvl = idx
			reloadTable(curBs)
		})
	}

	return vb
}

type logItem struct {
	s logStruct
	m map[string]any
}

type logTableH struct {
	items []logItem
}

func newLogTableHandler() *logTableH {
	m := new(logTableH)
	return m
}

func (h *logTableH) ColumnTypes(m *ui.TableModel) []ui.TableValue {
	return []ui.TableValue{

		ui.TableString(""),
	}
}

func (h *logTableH) NumRows(m *ui.TableModel) int {
	return len(h.items)
}

func (h *logTableH) getL(row int) string {
	s := h.items[row].s.L
	s = strings.TrimSpace(s)
	return s
}

func (h *logTableH) CellValue(m *ui.TableModel, row, column int) ui.TableValue {

	switch column {
	case 0:
		return ui.TableString(strconv.Itoa(row))
	case 1:
		return ui.TableString(h.getL(row))
	case 2:
		ts := h.items[row].s.T

		const longForm = "Jan 2, 2006 at 3:04pm (MST)"
		t, _ := time.Parse("060102 150405.000", ts)

		return ui.TableString(t.String())
	case 3:
		return ui.TableString(h.items[row].s.M)
	case 4:
		m := h.items[row].m
		if len(m) == 0 {
			return ui.TableString("")
		}
		ms := fmt.Sprintf("%v", m)
		ms = strings.TrimPrefix(ms, "map[")
		ms = strings.TrimSuffix(ms, "]")
		return ui.TableString(ms)

	case 15: //color for column 1
		switch h.getL(row) {
		case "INFO":
			return ui.TableColor{R: 0, G: 0, B: 1, A: 1}
		case "ERROR":
			return ui.TableColor{R: 1, G: 0, B: 0, A: 1}
		case "WARN":
			return ui.TableColor{R: 1, G: 1, B: 0, A: 1}
		case "DEBUG":
			return ui.TableColor{R: 0.5, G: 0.13, B: 0.75, A: 1}
		default:
			return ui.TableColor{R: 0, G: 0, B: 0, A: 1}
		}
	}

	return ui.TableString("")
}

func (h *logTableH) SetCellValue(m *ui.TableModel, row, column int, value ui.TableValue) {

}
