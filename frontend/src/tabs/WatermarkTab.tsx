import { useEffect, useRef, useState } from 'react';
import type { CSSProperties, PointerEvent as ReactPointerEvent } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import FilePicker from '../components/FilePicker';
import MediaInfoCard from '../components/MediaInfoCard';
import CommandPreview from '../components/CommandPreview';
import ProgressPanel from '../components/ProgressPanel';
import { api, formatTime } from '../api';
import AppSelect from '../ui/AppSelect';
import AppSlider from '../ui/AppSlider';
import AppNumber from '../ui/AppNumber';
import AppColor from '../ui/AppColor';
import AppCheck from '../ui/AppCheck';
import AppSwitch from '../ui/AppSwitch';
import { useTask } from '../hooks/useTask';
import { useMediaSource } from '../hooks/useMediaSource';
import { useCommandPreview } from '../hooks/useCommandPreview';
import { baseFields } from '../hooks/baseFields';
// 时间段计算与 Vue 版共用同一份实现（lib/watermarkTime.js，有独立的 node --test 用例）
import { clamp, spanOf, inRange, applyResize, applyMove, timeSummary, targetSeekFor } from '../lib/watermarkTime';
// 水印缩放的几何计算，同样有独立的 node --test 用例
import { scaleFromDrag, IMAGE_RATIO_RANGE, TEXT_RATIO_RANGE } from '../lib/watermarkLayout';

export default function WatermarkTab() {
  const { file, info, loadingInfo, probeError, outPath, setOutPath, previewURL, previewNote, preparing, load } =
    useMediaSource('watermark', { withPreview: true });

  const [items, setItems] = useState<any[]>([]);
  const [selectedId, setSelectedId] = useState('');
  const [targetMode, setTargetMode] = useState('source'); // source | 720 | 1080
  const seq = useRef(0);

  // 所有水印字段变更都走这里：React 状态不可变，禁止直接改 items[i].x
  function patchItem(id: string, patch: Record<string, any>) {
    setItems((prev) => prev.map((it) => (it.id === id ? { ...it, ...patch } : it)));
  }

  const selected = items.find((i) => i.id === selectedId) || null;
  const activeCount = items.filter((i) => i.enabled).length;

  // 删除后自动重选：函数式更新 + effect 兜底（闭包里的 items 可能已过期）
  useEffect(() => {
    if (selectedId && !items.some((i) => i.id === selectedId)) setSelectedId(items[0]?.id ?? '');
  }, [items, selectedId]);

  // 预览里的画面尺寸：位置和大小全部按比例存放，所以改输出分辨率也不会跑偏
  let outW = 1920;
  if (targetMode === '720') outW = 1280;
  else if (targetMode === '1080') outW = 1920;
  else outW = info?.video?.width || 1920;
  let outH = 1080;
  if (targetMode === '720') outH = 720;
  else if (targetMode === '1080') outH = 1080;
  else outH = info?.video?.height || 1080;

  const duration = info?.duration || 0;

  /* ---------- 预览舞台 ---------- */

  const stageEl = useRef<HTMLDivElement | null>(null);
  const stageBoxEl = useRef<HTMLDivElement | null>(null);
  const videoEl = useRef<HTMLVideoElement | null>(null);
  const tlEl = useRef<HTMLDivElement | null>(null);
  const [stageSize, setStageSize] = useState({ w: 0, h: 0 });
  const [currentTime, setCurrentTime] = useState(0);
  const [tlTip, setTlTip] = useState('');
  const tlTipTimer = useRef<any>(null);
  const roRef = useRef<ResizeObserver | null>(null);

  // 给 window 监听器 / pointerup 闭包用的最新值镜像（setItems 是异步的，闭包里的 state 会过期）
  const itemsRef = useRef(items);
  itemsRef.current = items;
  const currentTimeRef = useRef(currentTime);
  currentTimeRef.current = currentTime;

  // 预览舞台的像素尺寸。
  // 必须由 JS 精确算出，不能靠 CSS 撑开：只要舞台尺寸和视频内容区域有偏差，
  // 水印位置就会整体偏移，所见即所得就失效了。
  function relayout() {
    const box = stageBoxEl.current;
    if (!box) return;
    const avail = box.clientWidth || 640;
    const vw = info?.video?.width || 16;
    const vh = info?.video?.height || 9;
    const maxH = Math.min(window.innerHeight * 0.6, 500);

    let w = avail;
    let h = (w * vh) / vw;
    if (h > maxH) {
      h = maxH;
      w = (h * vw) / vh;
    }
    setStageSize((prev) => {
      const next = { w: Math.round(w), h: Math.round(h) };
      return prev.w === next.w && prev.h === next.h ? prev : next;
    });
  }
  const relayoutRef = useRef(relayout);
  relayoutRef.current = relayout;

  function observeBox() {
    if (roRef.current) roRef.current.disconnect();
    if (!stageBoxEl.current || typeof ResizeObserver === 'undefined') return;
    roRef.current = new ResizeObserver(() => relayoutRef.current());
    roRef.current.observe(stageBoxEl.current);
  }

  const videoW = info?.video?.width;
  const videoH = info?.video?.height;
  useEffect(() => {
    relayoutRef.current();
    observeBox();
    return () => {
      if (roRef.current) roRef.current.disconnect();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [previewURL, videoW, videoH]);

  useEffect(() => {
    const onResize = () => relayoutRef.current();
    window.addEventListener('resize', onResize);
    return () => {
      window.removeEventListener('resize', onResize);
      if (roRef.current) roRef.current.disconnect();
    };
  }, []);

  useEffect(() => () => clearTimeout(tlTipTimer.current), []);

  /* ---------- 时间段：每个水印各自独立 ---------- */

  // 时间轴上的临时提示：写给"拖了但没反应"的情况，
  // 否则用户只会以为这个功能坏了（撞到片头/片尾时本来就没有可拖的余地）。
  function showTip(msg: string) {
    setTlTip(msg);
    clearTimeout(tlTipTimer.current);
    tlTipTimer.current = setTimeout(() => {
      setTlTip('');
    }, 2800);
  }

  function onTimeUpdate() {
    setCurrentTime(videoEl.current?.currentTime || 0);
  }

  function isVisible(it: any) {
    return it.enabled && inRange(it, currentTime, duration);
  }

  function summaryOf(it: any) {
    return timeSummary(it, duration);
  }

  function barStyle(it: any): CSSProperties {
    const { start, end, duration: d } = spanOf(it, duration);
    const s = clamp(start, 0, d);
    const e = clamp(end, 0, d);
    return { left: (s / d) * 100 + '%', width: Math.max(1, ((e - s) / d) * 100) + '%' };
  }

  function playheadStyle(): CSSProperties {
    const d = duration || 1;
    return { left: clamp(currentTime / d, 0, 1) * 100 + '%' };
  }

  function seekTo(t: number) {
    const v = videoEl.current;
    if (v) v.currentTime = clamp(t, 0, duration || t);
    setCurrentTime(t);
  }

  // 时间轴上按住拖动 = 拖播放头（点一下也能直接跳过去）。
  // 必须挂在 window 上监听移动，否则鼠标一旦滑出色条/红线就断了。
  function startScrub(e: ReactPointerEvent | PointerEvent) {
    e.preventDefault();
    const tl = tlEl.current;
    if (!tl) return;
    const rect = tl.getBoundingClientRect();
    const d = duration || 0;
    if (!d || !rect.width) return;

    const apply = (clientX: number) => {
      const ratio = clamp((clientX - rect.left) / rect.width, 0, 1);
      const t = ratio * d;
      // 拖动过程会高频触发，差得不多就别反复 seek，免得视频一直重定位
      if (Math.abs(t - currentTimeRef.current) < 0.02) return;
      seekTo(t);
    };

    apply(e.clientX);
    const move = (ev: PointerEvent) => apply(ev.clientX);
    const up = () => {
      window.removeEventListener('pointermove', move);
      window.removeEventListener('pointerup', up);
      window.removeEventListener('pointercancel', up);
    };
    window.addEventListener('pointermove', move);
    window.addEventListener('pointerup', up);
    window.addEventListener('pointercancel', up);
  }

  // 调完时间段后，如果画面正停在这个水印看不到的时刻，就自动跳过去，
  // 省得用户还要手动拖进度条确认效果。已经在区间里则不动，避免打断播放。
  function afterTimeEdit(it: any) {
    // 用 id 查最新状态，避免调用方闭包里的 it 是过期快照
    const fresh = itemsRef.current.find((i) => i.id === it.id) ?? it;
    const t = videoEl.current?.currentTime ?? currentTimeRef.current;
    const target = targetSeekFor(fresh, t, duration);
    if (target !== null) seekTo(target);
  }

  // 色条内的文字：显示这段的时间范围，太窄时 CSS 会自然裁掉
  function barLabel(it: any) {
    if (!it.hasTime) return '全程';
    const { start, end } = spanOf(it, duration);
    return `${start.toFixed(1)} – ${end.toFixed(1)}`;
  }

  // 拖动色条两端的手柄 → 调整开始 / 消失时间
  function resizeBar(it: any, side: 'start' | 'end', e: ReactPointerEvent) {
    e.preventDefault();
    e.stopPropagation();
    setSelectedId(it.id);

    const tl = tlEl.current;
    if (!tl) return;
    const rect = tl.getBoundingClientRect();
    const d = duration || 1;
    // 以按下瞬间的区间为基准；否则每次 move 都基于上一帧结果累加，会越拖越快
    const base = spanOf(it, d);
    const wasLimited = !!it.hasTime;
    const startX = e.clientX;
    let activated = false;
    let latest = { start: base.start, end: base.end };

    const move = (ev: PointerEvent) => {
      const dx = ev.clientX - startX;
      // 位移太小一律当点击：不激活，也就不会改动数据（避免"点一下色条就变短"）
      if (!activated) {
        if (Math.abs(dx) < 3) return;
        activated = true;
        setItems((prev) => prev.map((x) => (x.id === it.id ? { ...x, hasTime: true } : x)));
      }

      const delta = (dx / rect.width) * d;
      const next = applyResize(base, side, delta, d);
      latest = next;
      setItems((prev) => prev.map((x) => (x.id === it.id ? { ...x, start: next.start, end: next.end } : x)));
    };

    const up = () => {
      window.removeEventListener('pointermove', move);
      window.removeEventListener('pointerup', up);
      window.removeEventListener('pointercancel', up);
      if (!activated) return;

      // 拖到头了就不动，但要说明原因 —— 否则看起来就是"拖不动"
      if (latest.start === base.start && latest.end === base.end) {
        // 没产生任何变化就把 hasTime 还原，免得平白留下一个"0–片尾"的限定
        if (!wasLimited) setItems((prev) => prev.map((x) => (x.id === it.id ? { ...x, hasTime: false } : x)));
        if (side === 'end') {
          showTip(
            base.end >= d - 0.001 ? '右端已是片尾，没法再往后拉；往左拖可以提前结束' : '区间已经缩到最短了'
          );
        } else {
          showTip(base.start <= 0.001 ? '左端已是片头，没法再往前；往右拖可以推迟出现' : '区间已经缩到最短了');
        }
        return;
      }
      afterTimeEdit({ ...it, ...latest, hasTime: true });
    };
    window.addEventListener('pointermove', move);
    window.addEventListener('pointerup', up);
    window.addEventListener('pointercancel', up);
  }

  // 拖动色条中段 → 整体平移（只影响它自己）。
  // 「全程」水印没有可平移的余地，所以给一句提示，免得用户以为拖不动。
  function moveBar(it: any, e: ReactPointerEvent) {
    e.preventDefault();
    e.stopPropagation();
    setSelectedId(it.id);

    if (!it.hasTime) {
      showTip('这是「全程」水印：拖色条两端的手柄，可以限定它出现的时间段');
      return;
    }

    const tl = tlEl.current;
    if (!tl) return;
    const rect = tl.getBoundingClientRect();
    const d = duration || 1;
    const base = spanOf(it, d);
    const startX = e.clientX;
    let activated = false;
    let latest = { start: base.start, end: base.end };

    const move = (ev: PointerEvent) => {
      const dx = ev.clientX - startX;
      if (!activated) {
        if (Math.abs(dx) < 3) return;
        activated = true;
      }
      const delta = (dx / rect.width) * d;
      const next = applyMove(base, delta, d);
      latest = next;
      setItems((prev) => prev.map((x) => (x.id === it.id ? { ...x, start: next.start, end: next.end } : x)));
    };

    const up = () => {
      window.removeEventListener('pointermove', move);
      window.removeEventListener('pointerup', up);
      window.removeEventListener('pointercancel', up);
      if (!activated) return;

      if (latest.start === base.start && latest.end === base.end) {
        showTip('这一段已经贴到片头或片尾了，没法再往这个方向平移');
        return;
      }
      afterTimeEdit({ ...it, ...latest });
    };
    window.addEventListener('pointermove', move);
    window.addEventListener('pointerup', up);
    window.addEventListener('pointercancel', up);
  }

  function clearTime(it: any) {
    patchItem(it.id, { hasTime: false, start: 0, end: 0 });
  }

  /* ---------- 拖拽定位 ---------- */
  function startDrag(item: any, e: ReactPointerEvent) {
    e.preventDefault();
    e.stopPropagation();
    setSelectedId(item.id);
    const stage = stageEl.current;
    if (!stage) return;
    const rect = stage.getBoundingClientRect();
    const sx = e.clientX;
    const sy = e.clientY;
    const ox = item.x;
    const oy = item.y;

    const move = (ev: PointerEvent) => {
      const nx = clamp(ox + (ev.clientX - sx) / rect.width, 0, 1);
      const ny = clamp(oy + (ev.clientY - sy) / rect.height, 0, 1);
      setItems((prev) => prev.map((x) => (x.id === item.id ? { ...x, x: nx, y: ny } : x)));
    };
    const up = () => {
      window.removeEventListener('pointermove', move);
      window.removeEventListener('pointerup', up);
      window.removeEventListener('pointercancel', up);
    };
    window.addEventListener('pointermove', move);
    window.addEventListener('pointerup', up);
    window.addEventListener('pointercancel', up);
  }

  function startResize(item: any, e: ReactPointerEvent) {
    e.preventDefault();
    e.stopPropagation();
    setSelectedId(item.id);

    // 量出水印元素当前的实际渲染宽度，作为整个拖动过程的缩放基准。
    // 文字水印的渲染宽度与字号并不成正比（三个字、字号 36px 时实际宽约 108px），
    // 直接拿字号当宽度算，手柄会跑得比鼠标快好几倍。
    const el = (e.currentTarget as HTMLElement)?.parentElement;
    const startWidth = el?.getBoundingClientRect().width || 0;
    const baseRatio = item.kind === 'image' ? item.wRatio : item.fontSizeRatio;
    const sx = e.clientX;

    const move = (ev: PointerEvent) => {
      const dx = ev.clientX - sx;
      if (item.kind === 'image') {
        const v = scaleFromDrag(baseRatio, startWidth, dx, ...IMAGE_RATIO_RANGE);
        setItems((prev) => prev.map((x) => (x.id === item.id ? { ...x, wRatio: v } : x)));
      } else {
        const v = scaleFromDrag(baseRatio, startWidth, dx, ...TEXT_RATIO_RANGE);
        setItems((prev) => prev.map((x) => (x.id === item.id ? { ...x, fontSizeRatio: v } : x)));
      }
    };

    const up = () => {
      window.removeEventListener('pointermove', move);
      window.removeEventListener('pointerup', up);
      window.removeEventListener('pointercancel', up);
    };
    window.addEventListener('pointermove', move);
    window.addEventListener('pointerup', up);
    window.addEventListener('pointercancel', up);
  }

  function stageStyle(item: any): CSSProperties {
    const base: CSSProperties = {
      left: `${item.x * 100}%`,
      top: `${item.y * 100}%`,
      cursor: 'move',
    };
    if (item.kind === 'image') {
      base.width = `${item.wRatio * 100}%`;
      base.opacity = item.opacity;
    } else {
      // 字号用像素写死，等价于「字号 ÷ 画面高」，和导出时的换算完全一致
      base.fontSize = `${Math.max(8, item.fontSizeRatio * stageSize.h)}px`;
      base.color = item.color;
      base.opacity = item.opacity;
      base.lineHeight = '1.2';
      if (item.borderW > 0) {
        (base as any).WebkitTextStroke = `${item.borderW * 0.5}px ${item.borderColor}`;
        base.textShadow = `0 0 ${item.borderW}px ${item.borderColor}`;
      }
      if (item.box) {
        base.background = 'rgba(0,0,0,0.5)';
        base.padding = '0.15em 0.4em';
        base.borderRadius = '4px';
      }
    }
    return base;
  }

  function addImage(wm: any) {
    const id = `w${++seq.current}`;
    // wm 来自 api.pickImageFile()：后端 PickedFile 含 url（json:"url"），仅前端预览用，后端会忽略
    setItems((prev) => [
      ...prev,
      {
        id,
        kind: 'image',
        enabled: true,
        path: wm.path,
        url: wm.url, // 仅前端预览用，后端会忽略
        x: 0.03,
        y: 0.03,
        wRatio: 0.18,
        opacity: 1,
        hasTime: false,
        start: 0,
        end: 0,
      },
    ]);
    setSelectedId(id);
  }

  function addText() {
    const id = `w${++seq.current}`;
    setItems((prev) => [
      ...prev,
      {
        id,
        kind: 'text',
        enabled: true,
        text: '示例文字',
        fontFile: '',
        fontSizeRatio: 0.05,
        color: '#ffffff',
        opacity: 0.95,
        borderW: 2,
        borderColor: '#000000',
        box: false,
        boxColor: 'black@0.5',
        boxBorderW: 8,
        x: 0.35,
        y: 0.78,
        hasTime: false,
        start: 0,
        end: 0,
      },
    ]);
    setSelectedId(id);
  }

  async function pickImage() {
    const f = await api.pickImageFile();
    if (f) addImage(f);
  }

  async function pickFont() {
    const f = await api.pickFontFile();
    if (f && selected) patchItem(selected.id, { fontFile: f.path });
  }

  function remove(id: string) {
    setItems((prev) => prev.filter((i) => i.id !== id));
  }

  function duplicate(id: string) {
    const src = items.find((i) => i.id === id);
    if (!src) return;
    const copy = {
      ...src,
      id: `w${++seq.current}`,
      x: clamp(src.x + 0.03, 0, 0.95),
      y: clamp(src.y + 0.03, 0, 0.95),
    };
    setItems((prev) => {
      const idx = prev.findIndex((i) => i.id === id);
      if (idx < 0) return prev;
      const next = [...prev];
      next.splice(idx + 1, 0, copy);
      return next;
    });
    setSelectedId(copy.id);
  }

  // 顺序即层叠顺序：靠后的盖在靠前的上面
  function move(id: string, dir: number) {
    setItems((prev) => {
      const i = prev.findIndex((x) => x.id === id);
      const j = i + dir;
      if (i < 0 || j < 0 || j >= prev.length) return prev;
      const next = [...prev];
      [next[i], next[j]] = [next[j], next[i]];
      return next;
    });
  }

  /* ---------- 生成任务 ---------- */
  function buildSpec() {
    if (!file || !outPath || !info) return null;
    if (!activeCount) return null;

    return {
      kind: 'watermark',
      output: outPath,
      ...baseFields(file, info),
      watermark: {
        targetW: targetMode === 'source' ? 0 : outW,
        targetH: targetMode === 'source' ? 0 : outH,
        // 只发后端认识的字段，url 是前端预览用的
        items: items.map((i) => ({
          kind: i.kind,
          enabled: i.enabled,
          path: i.path || '',
          text: i.text || '',
          fontFile: i.fontFile || '',
          fontSizeRatio: Number(i.fontSizeRatio) || 0,
          color: i.color || '',
          opacity: Number(i.opacity) || 0,
          borderW: Number(i.borderW) || 0,
          borderColor: i.borderColor || '',
          box: !!i.box,
          boxColor: i.boxColor || '',
          boxBorderW: Number(i.boxBorderW) || 0,
          x: Number(i.x) || 0,
          y: Number(i.y) || 0,
          wRatio: Number(i.wRatio) || 0,
          hasTime: !!i.hasTime,
          start: Number(i.start) || 0,
          end: Number(i.end) || 0,
        })),
      },
    };
  }

  const { cmd } = useCommandPreview(buildSpec, [file, outPath, items, targetMode, info]);
  const { running, progress, stage, logs, result, start, cancel } = useTask();

  async function run() {
    const spec = buildSpec();
    if (!spec) return;
    try {
      await start(spec);
    } catch (e: any) {
      window.alert('无法启动任务：' + (e?.message || e));
    }
  }

  async function pickOutput() {
    const ext = file?.name?.match(/\.[^.]+$/)?.[0] || '.mp4';
    const name = (file?.name || 'output').replace(/\.[^.]+$/, '') + '_watermarked' + ext;
    const p = await api.pickSaveFile(name, `*${ext}`);
    if (p) setOutPath(p);
  }

  return (
    <>
      <div className="card">
        <h2>源文件</h2>
        <FilePicker value={file} onChange={load} label="选择视频，也可以直接拖进来" />
        {file && (
          <div style={{ marginTop: 12 }}>
            <MediaInfoCard info={info} loading={loadingInfo} />
            {preparing ? (
              <div className="banner info" style={{ marginTop: 10 }}>
                正在准备预览…
              </div>
            ) : (
              previewNote && (
                <div className="banner info" style={{ marginTop: 10 }}>
                  {previewNote}
                </div>
              )
            )}
            {probeError && (
              <div className="banner warn" style={{ marginTop: 10 }}>
                {probeError}
              </div>
            )}
          </div>
        )}
      </div>

      {file && previewURL && (
        <div className="wm-layout">
          {/* 预览区：水印是叠在视频上的网页元素，拖动零延迟；导出时按同一套比例换算成 ffmpeg 参数 */}
          <div className="card stage-card">
            <h2>
              实时预览
              <span className="hint">拖水印移动位置，拖右下角圆点改大小</span>
            </h2>
            <div ref={stageBoxEl} className="stage-box">
              <div
                ref={stageEl}
                className="stage"
                data-testid="wm-canvas"
                style={{ width: stageSize.w + 'px', height: stageSize.h + 'px' }}
              >
                <video
                  ref={videoEl}
                  src={previewURL}
                  controls
                  preload="metadata"
                  onTimeUpdate={onTimeUpdate}
                  onSeeked={onTimeUpdate}
                />
                {items.map((it) => (
                  <div
                    key={it.id}
                    className={'wm' + (it.id === selectedId ? ' sel' : '') + (it.kind === 'text' ? ' text' : '')}
                    style={{ ...stageStyle(it), display: isVisible(it) ? undefined : 'none' }}
                    onPointerDown={(e) => startDrag(it, e)}
                  >
                    {it.kind === 'image' ? (
                      <img src={it.url} alt="水印" draggable={false} />
                    ) : (
                      <span>{it.text || '文字'}</span>
                    )}
                    <i className="resize" onPointerDown={(e) => startResize(it, e)}></i>
                  </div>
                ))}
              </div>
            </div>
            {/* 时间轴：每个水印一条色条，各自的时间段一眼可见、可直接拖动 */}
            <div className="tl">
              <div className="tl-head">
                <span className={'hint' + (tlTip ? ' warn' : '')}>
                  {tlTip || '时间轴 · 拖红点定位播放位置；拖色条两端调起止时间，拖中段平移'}
                </span>
                <span className="mono tl-clock">
                  {formatTime(currentTime)} / {formatTime(duration)}
                </span>
              </div>

              <div ref={tlEl} id="track" className="tl-track" onPointerDown={startScrub}>
                {!items.length && <div className="tl-empty">还没有水印</div>}
                {items.map((it) => (
                  <div
                    key={it.id}
                    className={
                      'tl-bar' +
                      (!it.enabled ? ' off' : '') +
                      (it.id === selectedId ? ' sel' : '') +
                      (it.kind === 'text' ? ' text' : '') +
                      (!it.hasTime ? ' full' : '')
                    }
                    data-testid={`tl-bar-${it.id}`}
                    style={barStyle(it)}
                    title={`${it.kind === 'image' ? '图片水印' : it.text || '文字水印'} · ${summaryOf(it)}`}
                    onPointerDown={(e) => moveBar(it, e)}
                  >
                    <span
                      className="tl-grip left"
                      data-testid={`tl-handle-${it.id}-left`}
                      title="拖动这里：调整开始时间"
                      onPointerDown={(e) => resizeBar(it, 'start', e)}
                    ></span>
                    <span className="tl-label">{barLabel(it)}</span>
                    <span
                      className="tl-grip right"
                      data-testid={`tl-handle-${it.id}-right`}
                      title="拖动这里：调整消失时间"
                      onPointerDown={(e) => resizeBar(it, 'end', e)}
                    ></span>
                  </div>
                ))}
                <div className="tl-playhead" data-testid="tl-playhead" style={playheadStyle()}>
                  <span
                    className="tl-knob"
                    title="按住拖动：移动播放位置"
                    onPointerDown={(e) => {
                      e.stopPropagation();
                      startScrub(e);
                    }}
                  ></span>
                </div>
              </div>

              <div className="tl-scale mono">
                <span>0:00</span>
                <span>{formatTime(duration / 2)}</span>
                <span>{formatTime(duration)}</span>
              </div>
            </div>

            <div className="stage-foot">
              <span className="hint">预览仅用于定位，最终效果以导出为准</span>
            </div>
          </div>

          {/* 右侧：水印列表 + 选中项属性 */}
          <div className="side">
            <div className="card">
              <h2>
                水印列表 <span className="hint">越靠后越在上层</span>
              </h2>

              {!items.length ? (
                <div className="empty-hint" style={{ padding: 18 }}>
                  还没有水印
                </div>
              ) : (
                <div className="list">
                  {/* 新增/删除/排序都带过渡，layout 让重排平滑滑动 */}
                  <AnimatePresence initial={false}>
                    {items.map((it, idx) => (
                      <motion.div
                        key={it.id}
                        layout
                        className={'listitem' + (it.id === selectedId ? ' on' : '')}
                        initial={{ opacity: 0, x: -12 }}
                        animate={{ opacity: 1, x: 0 }}
                        exit={{ opacity: 0, x: 12 }}
                        transition={{ duration: 0.15 }}
                        onClick={() => setSelectedId(it.id)}
                      >
                        <span onClick={(e) => e.stopPropagation()}>
                          <AppSwitch checked={!!it.enabled} label="启用水印" onChange={(v) => patchItem(it.id, { enabled: v })} />
                        </span>
                        <span className="li-body">
                          <span className="li-name">{it.kind === 'image' ? '图片水印' : it.text || '文字水印'}</span>
                          <span className={'li-time' + (it.hasTime ? ' limited' : '')}>{summaryOf(it)}</span>
                        </span>
                        <span className="li-ops">
                          <button className="ghost tiny" disabled={idx === 0} onClick={(e) => { e.stopPropagation(); move(it.id, -1); }} title="上移">
                            ↑
                          </button>
                          <button className="ghost tiny" disabled={idx === items.length - 1} onClick={(e) => { e.stopPropagation(); move(it.id, 1); }} title="下移">
                            ↓
                          </button>
                          <button className="ghost tiny" onClick={(e) => { e.stopPropagation(); duplicate(it.id); }} title="复制">
                            ⧉
                          </button>
                          <button className="ghost tiny" onClick={(e) => { e.stopPropagation(); remove(it.id); }} title="删除">
                            ×
                          </button>
                        </span>
                      </motion.div>
                    ))}
                  </AnimatePresence>
                </div>
              )}

              <div className="row" style={{ marginTop: 10 }}>
                <button className="tiny" onClick={pickImage}>
                  + 图片水印
                </button>
                <button className="tiny" onClick={addText}>
                  + 文字水印
                </button>
              </div>
            </div>

            {selected && (
              <div className="card">
                <h2>选中项设置</h2>

                {selected.kind === 'image' ? (
                  <div className="field">
                    <label>图片</label>
                    <div className="row">
                      <input value={selected.path} type="text" className="mono" style={{ flex: 1 }} readOnly />
                      <button className="tiny" onClick={pickImage}>
                        换一张
                      </button>
                    </div>
                  </div>
                ) : (
                  <>
                    <div className="field">
                      <label>文字内容</label>
                      <input
                        value={selected.text}
                        type="text"
                        placeholder="要显示的文字"
                        onChange={(e) => patchItem(selected.id, { text: e.target.value })}
                      />
                    </div>
                    <div className="field" style={{ marginTop: 10 }}>
                      <label>
                        字体文件 <span className="tip">留空则自动选系统字体</span>
                      </label>
                      <div className="row">
                        <input value={selected.fontFile || '（自动）'} type="text" className="mono" style={{ flex: 1 }} readOnly />
                        <button className="tiny" onClick={pickFont}>
                          选择…
                        </button>
                      </div>
                    </div>
                    <div className="grid2" style={{ marginTop: 10 }}>
                      <div className="field">
                        <label>文字颜色</label>
                        <AppColor value={selected.color || '#ffffff'} label="文字颜色" onChange={(v) => patchItem(selected.id, { color: v })} />
                      </div>
                      <div className="field">
                        <label>描边宽度</label>
                        <AppNumber value={selected.borderW} min={0} max={10} step={1} label="描边宽度" onChange={(v) => patchItem(selected.id, { borderW: v })} />
                      </div>
                    </div>
                    <div className="grid2" style={{ marginTop: 10 }}>
                      <div className="field">
                        <label>描边颜色</label>
                        <AppColor value={selected.borderColor || '#000000'} label="描边颜色" onChange={(v) => patchItem(selected.id, { borderColor: v })} />
                      </div>
                      <div className="field" style={{ justifyContent: 'flex-end', paddingBottom: 4 }}>
                        <AppCheck checked={!!selected.box} onChange={(v) => patchItem(selected.id, { box: v })}>
                          加背景框
                        </AppCheck>
                      </div>
                    </div>
                  </>
                )}

                <div className="divider"></div>

                <div className="field">
                  <label>不透明度 {Math.round(selected.opacity * 100)}%</label>
                  <AppSlider
                    value={selected.opacity}
                    min={0.05}
                    max={1}
                    step={0.01}
                    label="不透明度"
                    onChange={(v) => patchItem(selected.id, { opacity: v })}
                  />
                </div>

                {selected.hasTime && (
                  <>
                    <div className="divider"></div>
                    <div className="grid2">
                      <div className="field">
                        <label>起始（秒）</label>
                        <AppNumber
                          value={selected.start}
                          min={0}
                          step={0.1}
                          label="起点"
                          onChange={(v) => {
                            patchItem(selected.id, { start: v });
                            afterTimeEdit(selected);
                          }}
                        />
                      </div>
                      <div className="field">
                        <label>结束（秒，0 = 到片尾）</label>
                        <AppNumber
                          value={selected.end}
                          min={0}
                          step={0.1}
                          label="终点"
                          onChange={(v) => {
                            patchItem(selected.id, { end: v });
                            afterTimeEdit(selected);
                          }}
                        />
                      </div>
                    </div>
                    <div className="row" style={{ marginTop: 8 }}>
                      <button className="tiny" onClick={() => clearTime(selected)}>
                        取消时间限制
                      </button>
                    </div>
                  </>
                )}
              </div>
            )}

            <div className="card">
              <h2>输出设置</h2>
              <div className="grid2">
                <div className="field">
                  <label>输出分辨率</label>
                  <AppSelect
                    value={targetMode}
                    options={[
                      { value: 'source', label: '与源相同' },
                      { value: '720', label: '720p（1280×720）' },
                      { value: '1080', label: '1080p（1920×1080）' },
                    ]}
                    onChange={setTargetMode}
                  />
                </div>
                <div className="field">
                  <label>水印数量</label>
                  <div className="calc mono">{activeCount} 个生效</div>
                </div>
              </div>

              {activeCount > 5 && (
                <div className="banner warn" style={{ marginTop: 12, marginBottom: 0 }}>
                  水印较多，每个水印都要叠一次，编码速度会明显下降。
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {file && (
        <div className="card">
          <h2>输出</h2>
          <div className="field">
            <label>输出路径</label>
            <div className="row">
              <input value={outPath} onChange={(e) => setOutPath(e.target.value)} type="text" className="mono" style={{ flex: 1 }} />
              <button onClick={pickOutput}>浏览…</button>
            </div>
          </div>

          <CommandPreview steps={cmd.steps} err={cmd.err} />

          <div className="row" style={{ marginTop: 14 }}>
            <button className="primary" disabled={running || !outPath || !activeCount} onClick={run}>
              {running ? '处理中…' : `开始导出（${activeCount} 个水印）`}
            </button>
          </div>
        </div>
      )}

      <ProgressPanel
        running={running}
        progress={progress}
        stage={stage}
        logs={logs}
        result={result}
        onCancel={cancel}
        onOpenFile={api.openPath}
        onOpenDir={api.openDirectory}
      />
    </>
  );
}
