package task

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrBusy 表示已有任务在跑。转码本身就吃满 CPU，并行只会互相拖慢，所以串行执行。
var ErrBusy = errors.New("已有任务正在执行，请等它完成或先取消")

// Manager 管理任务的生命周期：同一时刻最多一个任务。
type Manager struct {
	mu        sync.Mutex
	running   bool
	currentID string
	cancel    context.CancelFunc
	seq       int64
}

func NewManager() *Manager {
	return &Manager{}
}

// Start 启动一个任务，返回任务 ID。
func (m *Manager) Start(ffmpegPath string, plan *Plan, hooks Hooks) (string, error) {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return "", ErrBusy
	}

	m.seq++
	id := fmt.Sprintf("%d-%d", time.Now().UnixNano(), m.seq)
	ctx, cancel := context.WithCancel(context.Background())

	m.running = true
	m.currentID = id
	m.cancel = cancel
	m.mu.Unlock()

	go func() {
		done := Run(ctx, id, ffmpegPath, plan, hooks)

		// 先释放占用再回调，否则前端收到 done 立刻提交下一个任务会被拒
		m.mu.Lock()
		m.running = false
		m.currentID = ""
		m.cancel = nil
		m.mu.Unlock()
		cancel()

		if hooks.OnDone != nil {
			hooks.OnDone(done)
		}
	}()

	return id, nil
}

// Cancel 取消正在执行的任务。id 为空表示取消当前任务。
func (m *Manager) Cancel(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return errors.New("当前没有正在执行的任务")
	}
	if id != "" && id != m.currentID {
		return errors.New("任务 ID 不匹配，可能已经结束")
	}
	m.cancel()
	return nil
}

// Busy 返回是否有任务在执行。
func (m *Manager) Busy() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

// CurrentID 返回当前任务的 ID，空闲时为空串。
func (m *Manager) CurrentID() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.currentID
}
