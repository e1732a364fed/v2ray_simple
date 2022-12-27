package vmess

import (
	"sync"
	"time"
)

const (
	//authid 120 seconds anti replay.
	authid_antiReplyDuration = time.Second * 120

	//在v2ray的代码中找到的。暂未找到对应的 “3分钟内不重复” 的文档。先照葫芦画瓢，到时候再追究。
	sessionAntiReplayDuration = time.Minute * 3
)

/*
我们用map的方式 实现 authid 防重放 机制. 不用v2ray的代码实现。

v2ray中 “使用 两个filter，每隔120秒swap一次filter” 的方式，感觉不够严谨。第121～240秒的话，
实际上 第一个 filter 仍然会存储之前所有数据，而只是第二个filter被重置了，导致 第一个filter是防240秒重放。
然后下一个120秒的话，又是 第二个 filter 防 240秒 重放，总之不太符合 标准定义。
因为这样的话，实际上把时间按120秒分块了，如果一个id在 第 1秒被使用，然后在第122秒被重新使用，按v2ray的实现，
依然会被认定为重放攻击。

我们只要用 v2ray的 sessionHistory的方式，存储过期时间，然后定时清理 即可。
*/
type authid_antiReplayMachine struct {
	sync.RWMutex
	authidMap map[[16]byte]time.Time //key: authid, value: expireTime

	ticker   *time.Ticker
	stopChan chan struct{}
	closed   bool
}

func newAuthIDAntiReplyMachine() *authid_antiReplayMachine {
	arm := &authid_antiReplayMachine{
		authidMap: make(map[[16]byte]time.Time),
		ticker:    time.NewTicker(authid_antiReplyDuration * 2),
		stopChan:  make(chan struct{}),
	}

	//定时清理过时数据，避免缓存无限增长
	go func(a *authid_antiReplayMachine) {

		for {
			select {
			case <-a.stopChan:
				return
			case now := <-a.ticker.C:
				a.Lock()
				for authid, expireTime := range a.authidMap {
					if expireTime.Before(now) {
						delete(a.authidMap, authid)
					}
				}
				a.Unlock()
			}
		}
	}(arm)
	return arm
}

func (arm *authid_antiReplayMachine) stop() {
	arm.Lock()
	defer arm.Unlock()

	if arm.closed {
		return
	}
	arm.closed = true
	close(arm.stopChan)
	arm.ticker.Stop()

}

type sessionID struct {
	user [16]byte
}
type session_antiReplayMachine struct {
	sync.RWMutex
	sessionMap map[sessionID]time.Time

	ticker   *time.Ticker
	stopChan chan struct{}
	closed   bool
}

func newSessionAntiReplayMachine() *session_antiReplayMachine {
	h := &session_antiReplayMachine{
		ticker:   time.NewTicker(sessionAntiReplayDuration * 2),
		stopChan: make(chan struct{}),
	}
	h.initCache()

	//定时清理过时数据，避免缓存无限增长
	go func(sh *session_antiReplayMachine) {

		for {
			select {
			case <-sh.stopChan:
				return
			case now := <-sh.ticker.C:
				sh.Lock()
				sh.removeExpiredEntries(now)
				sh.Unlock()
			}
		}
	}(h)

	return h
}

func (sh *session_antiReplayMachine) stop() {
	sh.Lock()
	defer sh.Unlock()

	if sh.closed {
		return
	}
	sh.closed = true
	close(sh.stopChan)
	sh.ticker.Stop()

}

func (h *session_antiReplayMachine) initCache() {
	h.sessionMap = make(map[sessionID]time.Time, 128)
}

// func (h *session_antiReplayMachine) check(session sessionID) bool {
// 	h.Lock()

// 	now := time.Now()

// 	if expire, found := h.sessionMap[session]; found && expire.After(now) {
// 		h.Unlock()
// 		return false
// 	}

// 	h.sessionMap[session] = now.Add(sessionAntiReplayDuration)
// 	h.Unlock()

// 	return true
// }

func (h *session_antiReplayMachine) removeExpiredEntries(now time.Time) {

	if len(h.sessionMap) == 0 {
		return
	}

	for session, expire := range h.sessionMap {
		if expire.Before(now) {
			delete(h.sessionMap, session)
		}
	}

	if len(h.sessionMap) == 0 {
		h.initCache() //这是为了回收内存。
	}

}
