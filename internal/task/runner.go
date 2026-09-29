package task

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"ffmpeg-studio/internal/proc"
)

// Progress 是一次进度上报。
type Progress struct {
	TaskID        string  `json:"taskId"`
	Percent       float64 `json:"percent"`
	StepIndex     int     `json:"stepIndex"`
	StepTotal     int     `json:"stepTotal"`
	StepLabel     string  `json:"stepLabel"`
	OutTime       float64 `json:"outTime"`
	Speed         string  `json:"speed"`
	FPS           float64 `json:"fps"`
	Frame         int64   `json:"frame"`
	SizeBytes     int64   `json:"sizeBytes"`
	ETASec        float64 `json:"etaSec"`
	Indeterminate bool    `json:"indeterminate"`
}

// Stage 表示多遍任务的阶段切换。
type Stage struct {
	TaskID string `json:"taskId"`
	Index  int    `json:"index"`
	Total  int    `json:"total"`
	Label  string `json:"label"`
}

// Done 是任务结束时的结果。
type Done struct {
	TaskID     string `json:"taskId"`
	OK         bool   `json:"ok"`
	Err        string `json:"err"`
	OutputPath string `json:"outputPath"`
	ElapsedMs  int64  `json:"elapsedMs"`
	Canceled   bool   `json:"canceled"`
}

// Hooks 是执行过程中的回调。
// runner 本身不依赖 Wails，由上层把回调接到事件推送上，这样 runner 可以独立测试。
type Hooks struct {
	OnProgress func(Progress)
	OnLog      func(string)
	OnStage    func(Stage)
	OnDone     func(Done)
}

// Run 按顺序执行计划里的每一步，直到完成或被取消。
func Run(ctx context.Context, taskID, ffmpegPath string, plan *Plan, hooks Hooks) Done {
	started := time.Now()
	done := Done{TaskID: taskID, OutputPath: plan.OutputPath}

	defer cleanupTemp(plan)

	var baseWeight float64
	for i, step := range plan.Steps {
		if ctx.Err() != nil {
			done.Err = "已取消"
			done.Canceled = true
			done.ElapsedMs = time.Since(started).Milliseconds()
			return done
		}

		if hooks.OnStage != nil {
			hooks.OnStage(Stage{TaskID: taskID, Index: i + 1, Total: len(plan.Steps), Label: step.Label})
		}
		if hooks.OnLog != nil {
			hooks.OnLog(fmt.Sprintf("▶ %s", step.Label))
		}

		if err := runStep(ctx, ffmpegPath, step, plan, i, baseWeight, hooks); err != nil {
			done.OK = false
			done.Err = err.Error()
			if ctx.Err() != nil {
				done.Canceled = true
			}
			done.ElapsedMs = time.Since(started).Milliseconds()
			return done
		}

		baseWeight += step.Weight
	}

	if hooks.OnProgress != nil {
		hooks.OnProgress(Progress{
			TaskID:    taskID,
			Percent:   100,
			StepIndex: len(plan.Steps),
			StepTotal: len(plan.Steps),
			StepLabel: plan.Steps[len(plan.Steps)-1].Label,
		})
	}
	if hooks.OnLog != nil {
		hooks.OnLog(fmt.Sprintf("✓ 完成，用时 %.1f 秒", time.Since(started).Seconds()))
	}

	done.OK = true
	done.ElapsedMs = time.Since(started).Milliseconds()
	return done
}

func runStep(ctx context.Context, ffmpegPath string, step Step, plan *Plan, stepIndex int, baseWeight float64, hooks Hooks) error {
	cmd := proc.CommandContext(ctx, ffmpegPath, step.Args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("无法启动 ffmpeg: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		readProgress(stdout, step, plan, stepIndex, baseWeight, hooks)
	}()
	go func() {
		defer wg.Done()
		readLogs(stderr, hooks)
	}()

	err = cmd.Wait()
	wg.Wait()

	if err != nil {
		if ctx.Err() != nil {
			return errors.New("已取消")
		}
		return fmt.Errorf("%s 执行失败", step.Label)
	}
	return nil
}

// readProgress 解析 `-progress pipe:1` 输出的 key=value 流。
// 比解析 stderr 里的人类可读进度稳定得多。
func readProgress(r io.Reader, step Step, plan *Plan, stepIndex int, baseWeight float64, hooks Hooks) {
	if hooks.OnProgress == nil {
		io.Copy(io.Discard, r)
		return
	}

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var p Progress
	for scanner.Scan() {
		key, value, ok := strings.Cut(strings.TrimSpace(scanner.Text()), "=")
		if !ok {
			continue
		}

		switch key {
		case "frame":
			p.Frame, _ = strconv.ParseInt(value, 10, 64)
		case "fps":
			p.FPS, _ = strconv.ParseFloat(value, 64)
		case "total_size":
			p.SizeBytes, _ = strconv.ParseInt(value, 10, 64)
		case "out_time_us":
			if us, err := strconv.ParseFloat(value, 64); err == nil && us >= 0 {
				p.OutTime = us / 1e6
			}
		case "speed":
			p.Speed = value
		case "progress":
			// 每遇到一个 progress 行代表一轮上报结束
			emitProgress(hooks, step, plan, stepIndex, baseWeight, p)
		}
	}
}

func emitProgress(hooks Hooks, step Step, plan *Plan, stepIndex int, baseWeight float64, p Progress) {
	total := plan.TotalWeight
	if total <= 0 {
		total = 1
	}

	p.TaskID = ""
	p.StepIndex = stepIndex + 1
	p.StepTotal = len(plan.Steps)
	p.StepLabel = step.Label
	p.Indeterminate = step.Indeterminate || step.Duration <= 0

	if !p.Indeterminate {
		ratio := p.OutTime / step.Duration
		if ratio > 1 {
			ratio = 1
		}
		p.Percent = (baseWeight + ratio*step.Weight) / total * 100
		p.ETASec = estimateETA(step.Duration-p.OutTime, p.Speed)
	} else {
		// 无法估算的步骤（例如只输出一帧）只报阶段起始位置
		p.Percent = baseWeight / total * 100
	}

	hooks.OnProgress(p)
}

// estimateETA 根据当前倍速估算剩余秒数。
func estimateETA(remain float64, speedText string) float64 {
	if remain <= 0 {
		return 0
	}
	speed := parseSpeed(speedText)
	if speed <= 0.01 {
		return 0
	}
	return remain / speed
}

func parseSpeed(s string) float64 {
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "x"))
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

// readLogs 把 ffmpeg 的 stderr 逐行转给前端。
// 已加 -hide_banner -nostats，这里拿到的都是实质信息。
func readLogs(r io.Reader, hooks Hooks) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r\n")
		if line == "" || hooks.OnLog == nil {
			continue
		}
		hooks.OnLog(line)
	}
}

// cleanupTemp 删除本次任务产生的中间文件。
func cleanupTemp(plan *Plan) {
	if plan == nil {
		return
	}
	for _, f := range plan.TempFiles {
		os.Remove(f)
	}
	for _, prefix := range plan.CleanupPrefixes {
		for _, f := range passLogFiles(prefix) {
			os.Remove(f)
		}
	}
}
