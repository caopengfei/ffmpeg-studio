import { useRef } from 'react';
import { Slider } from '@heroui/react';
import AppNumber from '../ui/AppNumber';
import { formatTime } from '../api';
import './VideoScrubber.css';

// 可视化时间轴：截取片段、抽帧、GIF 三个 Tab 共用。
// mode="range" 选区间（双滑块），mode="point" 选单点（单滑块）。
// 拖动逻辑交给 Slider（含键盘方向键、点击定位），这里只负责把值同步给视频预览
interface ScrubValue {
  start: number;
  end: number;
  time?: number;
}

interface VideoScrubberProps {
  src: string;
  duration: number;
  mode: 'range' | 'point';
  value: ScrubValue;
  onChange: (v: ScrubValue) => void;
  // range 模式是否显示起点/终点数字框（GIF 页隐藏，只留拖杆）
  showTimeFields?: boolean;
}

// HeroUI Slider 没有 min-steps-between-thumbs，按 Vue 的 5 步（step=0.01）
// 在 onChange 里手动保证双手柄最小间隔 0.05
const MIN_GAP = 0.05;

export default function VideoScrubber({ src, duration, mode, value, onChange, showTimeFields = true }: VideoScrubberProps) {
  const videoRef = useRef<HTMLVideoElement>(null);

  const startVal = mode === 'range' ? Number(value.start ?? 0) : 0;
  const endVal = mode === 'range' ? Number(value.end ?? duration) : Number(value.time ?? 0);

  // 时长还没读到时先给一个可用的刻度（要求 max > min）
  const maxVal = Math.max(duration || 0, 0.1);

  function seekTo(t: number) {
    const v = videoRef.current;
    if (v) v.currentTime = Math.max(0, Math.min(t, duration || t));
  }

  // 滑块拖动：找出是哪个手柄动了，视频实时跟到那个位置
  function onUpdate(arr: number[]) {
    if (!arr.length || arr.some((x) => typeof x !== 'number')) return;
    // 区间模式要双手柄的值；Slider 偶发只吐一个值时直接忽略，别把 end 写成 undefined
    if (mode === 'range' && arr.length < 2) return;

    if (mode === 'range') {
      let s = arr[0];
      let e = arr[1];
      const movedStart = Math.abs(s - startVal) >= Math.abs(e - endVal);
      if (e - s < MIN_GAP) {
        if (movedStart) s = Math.max(0, e - MIN_GAP);
        else e = Math.min(maxVal, s + MIN_GAP);
      }
      onChange({ ...value, start: s, end: e });
      seekTo(movedStart ? s : e);
    } else {
      onChange({ ...value, time: arr[0] });
      seekTo(arr[0]);
    }
  }

  function onMeta() {
    // 时长以探测结果为准，但视频元数据更准时同步一次；
    // duration 读不到（NaN）时不同步，避免把 end 写坏
    const v = videoRef.current;
    if (!v || !Number.isFinite(v.duration)) return;
    if (mode === 'range' && (!value.end || value.end > v.duration)) {
      onChange({ ...value, end: v.duration });
    }
  }

  function clamp(n: number): number {
    if (!isFinite(n) || n < 0) return 0;
    if (duration && n > duration) return duration;
    return n;
  }

  function setStart(val: number) {
    const n = Number(val) || 0;
    onChange({ ...value, start: clamp(n) });
    seekTo(n);
  }
  function setEnd(val: number) {
    const n = Number(val) || 0;
    onChange({ ...value, end: clamp(n) });
    seekTo(n);
  }
  function setPoint(val: number) {
    const n = Number(val) || 0;
    onChange({ ...value, time: clamp(n) });
    seekTo(n);
  }

  return (
    <div className="scrubber">
      <video ref={videoRef} className="preview" src={src} controls preload="metadata" onLoadedMetadata={onMeta} />

      <div className="track-row">
        <Slider
          className="track"
          value={mode === 'range' ? [startVal, endVal] : [endVal]}
          minValue={0}
          maxValue={maxVal}
          step={0.01}
          onChange={(v) => onUpdate(Array.isArray(v) ? v : [v])}
          aria-label={mode === 'range' ? '时间区间选择' : '时间点选择'}
        >
          <Slider.Track className="rail">
            <Slider.Fill className="sel" />
          </Slider.Track>
          {/* RAC 多手柄靠 index 区分：不传时两个手柄都绑 values[0]，
              终点手柄会叠在起点上（曾因此导致拖杆错位） */}
          <Slider.Thumb className="handle" index={0} aria-label="时间手柄" />
          {mode === 'range' && <Slider.Thumb className="handle" index={1} aria-label="结束时间" />}
        </Slider>
      </div>

      <div className="times">
        {mode === 'range' ? (
          <>
            {showTimeFields && (
              <>
                <div className="tfield">
                  <label>起点（秒）</label>
                  <AppNumber value={startVal} min={0} max={maxVal} step={0.01} label="起点" onChange={setStart} />
                </div>
                <div className="tfield">
                  <label>终点（秒）</label>
                  <AppNumber value={endVal} min={0} max={maxVal} step={0.01} label="终点" onChange={setEnd} />
                </div>
              </>
            )}
            <div className="readout">
              选中 <b>{formatTime(Math.max(0, endVal - startVal))}</b>
              <span className="dim">/ 全片 {formatTime(duration)}</span>
            </div>
          </>
        ) : (
          <>
            <div className="tfield">
              <label>时间点（秒）</label>
              <AppNumber value={endVal} min={0} max={maxVal} step={0.01} label="时间点" onChange={setPoint} />
            </div>
            <div className="readout">
              <b>{formatTime(endVal)}</b>
              <span className="dim">/ 全片 {formatTime(duration)}</span>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
