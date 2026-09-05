package ratelimit

import (
	"sync"
	"time"
)

// Window 是一个简单的按 key（IP）滑动窗口限频器。
// 每个 key 在窗口期内最多允许一次 Allow() 成功。
type Window struct {
	mu      sync.Mutex
	perIP   time.Duration
	lastHit map[string]time.Time
}

// NewWindow 构造一个窗口为 perIP 的限频器。
func NewWindow(perIP time.Duration) *Window {
	return &Window{
		perIP:   perIP,
		lastHit: make(map[string]time.Time),
	}
}

// Allow 判断 key 是否允许通过。窗口内第二次调用返回 false。
func (w *Window) Allow(key string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := time.Now()
	if last, ok := w.lastHit[key]; ok && now.Sub(last) < w.perIP {
		return false
	}
	w.lastHit[key] = now
	return true
}
