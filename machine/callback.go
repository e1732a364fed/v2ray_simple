package machine

type callbacks struct {
	toggle []func(int) //开关代理

	updated []func() //运行中的配置发生了变更
}

func (m *M) AddToggleCallback(f func(int)) {
	m.toggle = append(m.toggle, f)
}
func (m *M) callToggleFallback(e int) {
	if len(m.toggle) > 0 {
		for _, f := range m.toggle {
			f(e)
		}
	}
}

func (m *M) AddUpdatedCallback(f func()) {
	m.updated = append(m.updated, f)
}
func (m *M) callUpdatedFallback() {
	if len(m.updated) > 0 {
		for _, f := range m.updated {
			f()
		}
	}
}
