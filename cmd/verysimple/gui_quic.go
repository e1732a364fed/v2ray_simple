//go:build gui && !noquic

package main

import (
	"github.com/e1732a364fed/ui"
	"github.com/e1732a364fed/v2ray_simple/advLayer/quic"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
)

func init() {
	appVboxExtra = append(appVboxExtra, func(vbox *ui.Box) {

		//quic 手动挡调节, 0.2 ~ 1.5 映射到 0 ～ 100

		spinbox := ui.NewSpinbox(0, 100)
		slider := ui.NewSlider(0, 100)

		vbox.Append(ui.NewLabel("调节hysteria手动挡 (数值越大, 速度越慢, 数值越小, 速度越快)"), false)

		setQuicManual := func(num float64) {
			quic.TheCustomRate = 0.2 + num/100*(1.5-0.2)
			if ce := utils.CanLogInfo("Manually Adjust Quic Hysteria Custom Rate"); ce != nil {
				ce.Write(zap.Float64("rate", quic.TheCustomRate))
			}
		}
		spinbox.OnChanged(func(*ui.Spinbox) {
			v := spinbox.Value()
			slider.SetValue(v)
			setQuicManual(float64(v))
		})
		slider.OnChanged(func(*ui.Slider) {
			v := slider.Value()
			spinbox.SetValue(v)

			setQuicManual(float64(v))

		})
		curV := int((quic.TheCustomRate - 0.2) * 100)
		slider.SetValue(curV)
		spinbox.SetValue(curV)
		vbox.Append(spinbox, false)
		vbox.Append(slider, false)
	})
}
