package quic

import (
	"time"

	"github.com/lucas-clemente/quic-go/congestion"
)

var TheCustomRate = 0.75

type BrutalSender_M struct {
	rttStats        congestion.RTTStatsProvider
	bps             congestion.ByteCount
	maxDatagramSize congestion.ByteCount
	pacer           *pacer
}

func NewBrutalSender_M(bps congestion.ByteCount) *BrutalSender_M {
	bs := &BrutalSender_M{
		bps:             bps,
		maxDatagramSize: initMaxDatagramSize,
	}
	bs.pacer = newPacer(func() congestion.ByteCount {
		return congestion.ByteCount(float64(bs.bps) / bs.getAckRate())
	})
	return bs
}

func (b *BrutalSender_M) SetRTTStatsProvider(rttStats congestion.RTTStatsProvider) {
	b.rttStats = rttStats
}

func (b *BrutalSender_M) TimeUntilSend(bytesInFlight congestion.ByteCount) time.Time {
	return b.pacer.TimeUntilSend()
}

func (b *BrutalSender_M) HasPacingBudget() bool {
	return b.pacer.Budget(time.Now()) >= b.maxDatagramSize
}

func (b *BrutalSender_M) CanSend(bytesInFlight congestion.ByteCount) bool {
	return bytesInFlight < b.GetCongestionWindow()
}

func (b *BrutalSender_M) GetCongestionWindow() congestion.ByteCount {
	rtt := maxDuration(b.rttStats.LatestRTT(), b.rttStats.SmoothedRTT())
	if rtt <= 0 {
		return 10240
	}
	return congestion.ByteCount(float64(b.bps) * rtt.Seconds() * 1.5 / b.getAckRate())
}

func (b *BrutalSender_M) OnPacketSent(sentTime time.Time, bytesInFlight congestion.ByteCount,
	packetNumber congestion.PacketNumber, bytes congestion.ByteCount, isRetransmittable bool) {
	b.pacer.SentPacket(sentTime, bytes)
}

func (b *BrutalSender_M) OnPacketAcked(number congestion.PacketNumber, ackedBytes congestion.ByteCount, priorInFlight congestion.ByteCount, eventTime time.Time) {
}

func (b *BrutalSender_M) OnPacketLost(number congestion.PacketNumber, lostBytes congestion.ByteCount, priorInFlight congestion.ByteCount) {
}

func (b *BrutalSender_M) SetMaxDatagramSize(size congestion.ByteCount) {
	b.maxDatagramSize = size
	b.pacer.SetMaxDatagramSize(size)
}

func rateOk(r float64) int {
	if r < 0.2 {
		return -1
	}
	if r > 1.5 {
		return 1
	}
	return 0
}

//原来最小值是0.75, 最大值是1，越小的话发包越疯狂.
// 我们改成最小值0.2, 最快可以 7.5倍发包
// 最大值改成 1.5， 这样最慢可以1倍速正常发包
func (b *BrutalSender_M) getAckRate() float64 {

	r := rateOk(TheCustomRate)
	switch r {
	case 0:
		return TheCustomRate
	case -1:
		return 0.2
	case 1:
		return 1.5
	default:
		panic("rateOk returned value not 0,-1,1")
	}

}

func (b *BrutalSender_M) InSlowStart() bool {
	return false
}

func (b *BrutalSender_M) InRecovery() bool {
	return false
}

func (b *BrutalSender_M) MaybeExitSlowStart() {}

func (b *BrutalSender_M) OnRetransmissionTimeout(packetsRetransmitted bool) {}
