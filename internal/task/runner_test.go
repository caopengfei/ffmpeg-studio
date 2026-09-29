package task

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ffmpeg-studio/internal/media"
)

func transcodeSpec(src, out string, info *media.MediaInfo) *Spec {
	s := sourceSpec(KindTranscode, src, out, info)
	s.Video = &VideoSettings{Codec: "libx264", CRF: 35, Preset: "ultrafast", PixFmt: "yuv420p"}
	s.Audio = &AudioSettings{Codec: "aac", BitRate: "64k"}
	return s
}

func TestRunReportsMonotonicProgress(t *testing.T) {
	ff := locateFFmpeg(t)
	dir := t.TempDir()
	src := makeSource(t, ff, dir)
	info, _ := media.Probe(ff.FFprobePath, src)

	plan, err := BuildPlan(transcodeSpec(src, filepath.Join(dir, "out.mp4"), info))
	if err != nil {
		t.Fatal(err)
	}

	var percents []float64
	var logs []string

	done := Run(context.Background(), "t1", ff.FFmpegPath, plan, Hooks{
		OnProgress: func(p Progress) { percents = append(percents, p.Percent) },
		OnLog:      func(l string) { logs = append(logs, l) },
	})

	if !done.OK {
		t.Fatalf("任务失败: %s", done.Err)
	}
	if len(percents) == 0 {
		t.Fatal("没有收到任何进度上报")
	}
	if got := percents[len(percents)-1]; got != 100 {
		t.Errorf("最后一次进度应为 100，实际 %v", got)
	}

	// 进度必须单调不减，否则进度条会往回跳
	for i := 1; i < len(percents); i++ {
		if percents[i] < percents[i-1]-0.01 {
			t.Errorf("进度回退：第 %d 次 %.2f < 第 %d 次 %.2f", i+1, percents[i], i, percents[i-1])
			break
		}
	}
	if len(logs) == 0 {
		t.Error("没有收到日志")
	}
	if done.ElapsedMs <= 0 {
		t.Error("未记录耗时")
	}
}

func TestRunReportsStagesForTwoPass(t *testing.T) {
	ff := locateFFmpeg(t)
	dir := t.TempDir()
	src := makeSource(t, ff, dir)
	info, _ := media.Probe(ff.FFprobePath, src)

	s := sourceSpec(KindCompress, src, filepath.Join(dir, "out.mp4"), info)
	s.Compress = &CompressSettings{Mode: "targetSize", TargetMB: 0.08, AudioKbps: 48}

	plan, _ := BuildPlan(s)

	var stages []Stage
	done := Run(context.Background(), "t2", ff.FFmpegPath, plan, Hooks{
		OnStage: func(st Stage) { stages = append(stages, st) },
	})

	if !done.OK {
		t.Fatalf("两遍编码失败: %s", done.Err)
	}
	if len(stages) != 2 {
		t.Fatalf("应上报 2 个阶段，实际 %d", len(stages))
	}
	for i, st := range stages {
		if st.Index != i+1 || st.Total != 2 {
			t.Errorf("第 %d 个阶段序号不对: %+v", i+1, st)
		}
	}
}

func TestRunCancelStopsTask(t *testing.T) {
	ff := locateFFmpeg(t)
	dir := t.TempDir()

	// 造一个稍长的源，用慢速 preset 让任务跑得够久，便于中途取消
	src := filepath.Join(dir, "src.mp4")
	runFFmpeg(t, ff.FFmpegPath, []string{
		"-y", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc=d=30:s=640x480:r=30",
		"-f", "lavfi", "-i", "sine=f=440:d=30",
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-shortest", src,
	}, "生成较长测试源")

	info, _ := media.Probe(ff.FFprobePath, src)

	s := sourceSpec(KindTranscode, src, filepath.Join(dir, "out.mp4"), info)
	s.Video = &VideoSettings{Codec: "libx264", CRF: 18, Preset: "veryslow", PixFmt: "yuv420p"}
	s.Audio = &AudioSettings{Codec: "aac", BitRate: "192k"}

	plan, _ := BuildPlan(s)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(700 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	done := Run(ctx, "t3", ff.FFmpegPath, plan, Hooks{})
	elapsed := time.Since(start)

	if !done.Canceled {
		t.Errorf("应标记为已取消，实际 ok=%v err=%q", done.OK, done.Err)
	}
	if done.OK {
		t.Error("取消的任务不该标记为成功")
	}
	if elapsed > 15*time.Second {
		t.Errorf("取消后应尽快退出，实际用了 %v", elapsed)
	}
}

func TestRunCleansTempFiles(t *testing.T) {
	ff := locateFFmpeg(t)
	dir := t.TempDir()
	src := makeSource(t, ff, dir)
	info, _ := media.Probe(ff.FFprobePath, src)

	s := sourceSpec(KindGif, src, filepath.Join(dir, "out.gif"), info)
	s.Gif = &GifSettings{Start: 0, End: 2, FPS: 8, Width: 120, TwoPass: true, Loop: -1}

	plan, _ := BuildPlan(s)
	if len(plan.TempFiles) == 0 {
		t.Fatal("两遍 GIF 应登记调色板临时文件")
	}
	palette := plan.TempFiles[0]

	done := Run(context.Background(), "t4", ff.FFmpegPath, plan, Hooks{})
	if !done.OK {
		t.Fatalf("GIF 任务失败: %s", done.Err)
	}

	if _, err := os.Stat(palette); err == nil {
		t.Errorf("调色板临时文件未被清理: %s", palette)
	}
}

// 两遍编码的 passlog 也要清掉，否则临时目录会越积越多
func TestRunCleansPassLogs(t *testing.T) {
	ff := locateFFmpeg(t)
	dir := t.TempDir()
	src := makeSource(t, ff, dir)
	info, _ := media.Probe(ff.FFprobePath, src)

	s := sourceSpec(KindCompress, src, filepath.Join(dir, "out.mp4"), info)
	s.Compress = &CompressSettings{Mode: "targetSize", TargetMB: 0.08, AudioKbps: 48}

	plan, _ := BuildPlan(s)
	if len(plan.CleanupPrefixes) == 0 {
		t.Fatal("未登记 passlog 前缀")
	}

	var before int
	for _, pfx := range plan.CleanupPrefixes {
		before += len(passLogFiles(pfx))
	}

	done := Run(context.Background(), "t5", ff.FFmpegPath, plan, Hooks{})
	if !done.OK {
		t.Fatalf("任务失败: %s", done.Err)
	}

	var after int
	for _, pfx := range plan.CleanupPrefixes {
		after += len(passLogFiles(pfx))
	}
	if after >= before && after > 0 {
		t.Errorf("passlog 未被清理，剩余 %d 个", after)
	}
}

func TestCancelWithoutRunningTask(t *testing.T) {
	m := NewManager()
	if err := m.Cancel(""); err == nil {
		t.Error("空闲时取消应当报错")
	}
}

func TestManagerRejectsConcurrentTasks(t *testing.T) {
	m := NewManager()

	// 直接占用槽位，模拟任务执行中
	m.mu.Lock()
	m.running = true
	m.currentID = "busy"
	m.cancel = func() {}
	m.mu.Unlock()

	if _, err := m.Start("ffmpeg", &Plan{}, Hooks{}); err != ErrBusy {
		t.Errorf("应返回 ErrBusy，实际 %v", err)
	}
	if !m.Busy() {
		t.Error("Busy 应为 true")
	}
}
