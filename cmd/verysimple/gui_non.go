//go:build !gui

package main

// placeholder
type GuiPreference struct{}

//img table code:

// nw := ui.NewWindow("img", 320, 320, false)
// uiimg := ui.NewImage(320, 320)
// rect := image.Rect(0, 0, 320, 320)
// rgbaImg := image.NewRGBA(rect)
// draw.Draw(rgbaImg, rect, qr.Image(256), image.Point{}, draw.Over)
// uiimg.Append(rgbaImg)
// const qrname = "vs_qrcode.png"
// qr.WriteFile(256, qrname)
// utils.OpenFile(qrname)

//nw.SetChild(img)
// mh := newImgTableHandler()
// mh.img = uiimg
// model := ui.NewTableModel(mh)

// table := ui.NewTable(&ui.TableParams{
// 	Model:                         model,
// 	RowBackgroundColorModelColumn: 3,
// })
// table.AppendImageColumn("QRCode", 0)
// nw.SetChild(table)
// nw.SetMargined(true)
// nw.OnClosing(func(w *ui.Window) bool { return true })
// nw.Show()

// type imgTableH struct {
// 	img *ui.Image
// }

// func newImgTableHandler() *imgTableH {
// 	m := new(imgTableH)
// 	return m
// }

// func (mh *imgTableH) ColumnTypes(m *ui.TableModel) []ui.TableValue {
// 	return []ui.TableValue{

// 		ui.TableImage{}, // column 1 image

// 	}
// }

// func (mh *imgTableH) NumRows(m *ui.TableModel) int {
// 	return 1
// }

// func (mh *imgTableH) CellValue(m *ui.TableModel, row, column int) ui.TableValue {

// 	return ui.TableImage{I: mh.img}

// }

// func (mh *imgTableH) SetCellValue(m *ui.TableModel, row, column int, value ui.TableValue) {

// }
