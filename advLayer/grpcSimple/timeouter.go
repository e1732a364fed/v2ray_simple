package grpcSimple

import (
	"net"
	"time"
)

type timeouter struct {
	deadline *time.Timer

	closeFunc func()
}

func (g *timeouter) LocalAddr() net.Addr                { return nil }
func (g *timeouter) RemoteAddr() net.Addr               { return nil }
func (g *timeouter) SetReadDeadline(t time.Time) error  { return g.SetDeadline(t) }
func (g *timeouter) SetWriteDeadline(t time.Time) error { return g.SetDeadline(t) }

func (g *timeouter) SetDeadline(t time.Time) error {

	var d time.Duration

	if g.deadline != nil {

		if t == (time.Time{}) {
			g.deadline.Stop()
			return nil
		}

		g.deadline.Reset(d)
		return nil
	} else {
		if t == (time.Time{}) {
			return nil
		}
		d = time.Until(t)

	}

	g.deadline = time.AfterFunc(d, g.closeFunc)
	return nil
}
