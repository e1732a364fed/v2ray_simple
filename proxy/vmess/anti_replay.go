package vmess

import (
	"sync"
	"time"
)

//authid 120 second anti replay.
const authid_antiReplyDuration = time.Second * 120

/*
	我们用自己的代码 实现 authid 防重放 机制. 不用v2ray的代码实现。

	v2ray中 “使用 两个filter，每隔120秒swap一次filter” 的方式，感觉不够严谨。第121～240秒的话，
	实际上 第一个 filter 仍然会存储之前所有数据，而只是第二个filter被重置了，导致 第一个filter是防240秒重放。
	然后下一个120秒的话，又是 第二个 filter 防 240秒 重放，总之不太符合 标准定义。
	因为这样的话，实际上把时间按120秒分块了，如果一个id在 第 1秒被使用，然后在第122秒被重新使用，按v2ray的实现，
	依然会被认定为重放攻击。

	我们只要和 sessionHistory的方式一样，存储过期时间，然后定时清理 即可。
*/
type anitReplayMachine struct {
	sync.RWMutex
	antiReplyMap map[[16]byte]time.Time //key: authid, value: expireTime

	ticker   *time.Ticker
	stopChan chan struct{}
	closed   bool
}

func newAntiReplyMachine() *anitReplayMachine {
	arm := &anitReplayMachine{
		antiReplyMap: make(map[[16]byte]time.Time),
		ticker:       time.NewTicker(authid_antiReplyDuration * 2),
		stopChan:     make(chan struct{}),
	}

	//定时清理过时数据，避免缓存无限增长
	go func(a *anitReplayMachine) {

		for {
			select {
			case <-a.stopChan:
				return
			case now := <-a.ticker.C:
				a.Lock()
				for authid, expireTime := range a.antiReplyMap {
					if expireTime.Before(now) {
						delete(a.antiReplyMap, authid)
					}
				}
				a.Unlock()
			}
		}
	}(arm)
	return arm
}

func (arm *anitReplayMachine) stop() {
	arm.Lock()
	defer arm.Unlock()

	if arm.closed {
		return
	}
	arm.closed = true
	close(arm.stopChan)
	arm.ticker.Stop()

}

func (arm *anitReplayMachine) check(authid [16]byte) (ok bool) {
	now := time.Now()
	arm.RLock()
	expireTime, has := arm.antiReplyMap[authid]
	arm.RUnlock()

	if !has {
		arm.Lock()
		arm.antiReplyMap[authid] = now.Add(authid_antiReplyDuration)
		arm.Unlock()

		return true
	}
	if expireTime.Before(now) {
		arm.Lock()
		arm.antiReplyMap[authid] = now.Add(authid_antiReplyDuration)
		arm.Unlock()

		return true
	}
	return false
}

type sessionID struct {
	user  [16]byte
	key   [16]byte
	nonce [16]byte
}
type sessionHistory struct {
	sync.RWMutex
	cache map[sessionID]time.Time
}

func NewSessionHistory() *sessionHistory {
	h := &sessionHistory{}
	h.initCache()

	return h
}

func (h *sessionHistory) initCache() {
	h.cache = make(map[sessionID]time.Time, 128)
}

func (h *sessionHistory) addIfNotExits(session sessionID) bool {
	h.Lock()

	now := time.Now()

	h.removeExpiredEntries(now)

	if expire, found := h.cache[session]; found && expire.After(now) {
		h.Unlock()
		return false
	}

	h.cache[session] = time.Now().Add(time.Minute * 3) //在v2ray的代码中找到的。暂未找到对应的 “3分钟内不重复” 的文档。
	h.Unlock()

	return true
}

func (h *sessionHistory) removeExpiredEntries(now time.Time) {

	if len(h.cache) == 0 {
		return
	}

	for session, expire := range h.cache {
		if expire.Before(now) {
			delete(h.cache, session)
		}
	}

	if len(h.cache) == 0 {
		h.initCache() //这是为了回收内存。
	}

}
