package machine

type callbacks struct {
	toggle []func(int)
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
