//go:build gui

package main

import (
	"net/netip"
	"os"
	"strconv"

	"github.com/e1732a364fed/ui"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
)

func makeRoutePage() ui.Control {

	vb := ui.NewVerticalBox()
	vb.SetPadded(true)

	vb.Append(ui.NewLabel("目前vs_gui只支持路由单项配置,不支持数组; 若要使用数组, 请手动编辑配置文件"), false)
	vb.Append(ui.NewLabel("编辑字符串时, 请确保按回车确认. 之后还要点击提交更改. 否则更改不会生效"), false)

	newBtn := ui.NewButton("新增")
	rmBtn := ui.NewButton("删除")
	submitBtn := ui.NewButton("提交更改")
	detailBtn := ui.NewButton("详细说明")

	hb := ui.NewHorizontalBox()
	hb.SetPadded(true)

	hb.Append(newBtn, false)
	hb.Append(rmBtn, false)
	hb.Append(submitBtn, false)
	hb.Append(detailBtn, false)

	vb.Append(hb, false)

	rth := newRouteTableHandler()
	rp := mainM.GetRoutePolicy()

	if rp != nil {
		rpn := rp.Clone()
		rth.RoutePolicy = rpn
	}
	model := ui.NewTableModel(rth)

	curSelectedIdx := -1

	table := ui.NewTable(&ui.TableParams{
		Model:                         model,
		RowBackgroundColorModelColumn: 3,
	})
	table.AppendTextColumn("idx", 0, 0, nil)
	table.AppendTextColumn("ToTag", 1, 1, nil)
	table.AppendTextColumn("User", 2, 2, nil)
	table.AppendTextColumn("IP", 3, 3, nil)
	table.AppendTextColumn("Match", 4, 4, nil)
	table.AppendTextColumn("Domain", 5, 5, nil)

	table.SetColumnWidth(0, 20)

	table.OnRowClicked(func(t *ui.Table, i int) {
		curSelectedIdx = i
	})

	vb.Append(table, false)

	newBtn.OnClicked(func(b *ui.Button) {
		rth.List = append(rth.List, netLayer.NewFullRouteSet())
		model.RowInserted(len(rth.List) - 1)
	})
	rmBtn.OnClicked(func(b *ui.Button) {
		if curSelectedIdx < 0 || curSelectedIdx >= len(rth.List) {
			return
		}
		rth.List = utils.TrimSlice(rth.List, curSelectedIdx)
		model.RowDeleted(curSelectedIdx)
	})
	submitBtn.OnClicked(func(b *ui.Button) {
		rpn := rth.RoutePolicy.Clone()
		mainM.SetRoutePolicy(&rpn)
		mainM.PrintAllStateForHuman(os.Stdout, true)
	})

	detailBtn.OnClicked(func(b *ui.Button) {
		ui.MsgBox(mainwin,
			"路由配置详细说明",
			`
match 为域名匹配任意字符串,
domain 匹配域名或子域名

每一项在配置文件中都可以提供数组, 而在本gui中只能配置一个.如果你编辑了原来的数组配置, 则原数据不会被保存
			`)
	})

	return vb
}

type routeTableH struct {
	netLayer.RoutePolicy
}

func newRouteTableHandler() *routeTableH {
	m := new(routeTableH)
	return m
}

func (rt *routeTableH) ColumnTypes(m *ui.TableModel) []ui.TableValue {
	return []ui.TableValue{

		ui.TableString(""),
	}
}

func (rt *routeTableH) NumRows(m *ui.TableModel) int {
	return len(rt.List)
}

func (rt *routeTableH) CellValue(m *ui.TableModel, row, column int) ui.TableValue {

	switch column {
	case 0:
		return ui.TableString(strconv.Itoa(row))
	case 1:
		return ui.TableString(rt.List[row].OutTag)
	case 2:
		us := rt.List[row].Users
		if len(us) > 0 {
			u := utils.GetOneFromMap(us, "")
			return ui.TableString(u)
		}
		return ui.TableString("")
	case 3:
		ips := rt.List[row].IPs
		if len(ips) > 0 {
			addr := utils.GetOneFromMap(ips, netip.Addr{})
			return ui.TableString(addr.String())
		}
		return ui.TableString("")

	case 4:
		ms := rt.List[row].Match
		if len(ms) > 0 {
			d := ms[0]
			return ui.TableString(d)
		}
		return ui.TableString("")
	case 5:
		domains := rt.List[row].Domains
		if len(domains) > 0 {
			d := utils.GetOneFromMap(domains, "")
			return ui.TableString(d)
		}
		return ui.TableString("")

	}

	return ui.TableInt(0)
}

func (rt *routeTableH) SetCellValue(m *ui.TableModel, row, column int, value ui.TableValue) {
	if ce := utils.CanLogDebug("gui: SetCellValue"); ce != nil {
		ce.Write(zap.Int("row", row),
			zap.Int("column", column),
			zap.Any("value", value),
		)
	}
	switch column {
	case 1:
		s := value.(ui.TableString)
		rt.List[row].OutTag = string(s)
	case 2:
		s := value.(ui.TableString)
		rt.List[row].Users = map[string]bool{string(s): true}
	case 3:
		s := value.(ui.TableString)

		na, e := netip.ParseAddr(string(s))
		if e == nil {
			rt.List[row].IPs = map[netip.Addr]bool{na: true}
		} else {
			if ce := utils.CanLogErr("gui: ip wrong"); ce != nil {
				ce.Write(zap.Error(e),
					zap.String("value", string(s)),
				)
			}
		}
	case 4:
		s := value.(ui.TableString)
		rt.List[row].Match = []string{string(s)}

	case 5:
		s := value.(ui.TableString)
		rt.List[row].Domains = map[string]bool{string(s): true}

	}
}
