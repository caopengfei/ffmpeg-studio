import { useEffect, useState } from 'react';
import { Input, ToggleButton, ToggleButtonGroup } from '@heroui/react';
import FilePicker from '../components/FilePicker';
import MediaInfoCard from '../components/MediaInfoCard';
import CommandPreview from '../components/CommandPreview';
import ProgressPanel from '../components/ProgressPanel';
import { api } from '../api';
import AppSelect from '../ui/AppSelect';
import AppNumber from '../ui/AppNumber';
import AppCheck from '../ui/AppCheck';
import { useTask } from '../hooks/useTask';
import { useMediaSource } from '../hooks/useMediaSource';
import { useCommandPreview } from '../hooks/useCommandPreview';
import { baseFields } from '../hooks/baseFields';

const PRESETS = [
  { key: '2160', label: '4K · 2160p', w: 3840, h: 2160 },
  { key: '1440', label: '2K · 1440p', w: 2560, h: 1440 },
  { key: '1080', label: '1080p', w: 1920, h: 1080 },
  { key: '720', label: '720p', w: 1280, h: 720 },
  { key: '480', label: '480p', w: 854, h: 480 },
  { key: '360', label: '360p', w: 640, h: 360 },
  { key: 'custom', label: '自定义', w: 0, h: 0 },
];

const FLAG_OPTIONS = [
  { value: 'lanczos', label: 'lanczos（画质最好，推荐）' },
  { value: 'bicubic', label: 'bicubic' },
  { value: 'bilinear', label: 'bilinear' },
  { value: 'area', label: 'area（缩小最干净）' },
  { value: 'neighbor', label: 'neighbor（最快，锯齿明显）' },
];

export default function ResizeTab() {
  const { file, info, loadingInfo, probeError, outPath, setOutPath, load } = useMediaSource('resize');

  const [preset, setPreset] = useState('720');
  const [width, setWidth] = useState(1280);
  const [height, setHeight] = useState(720);
  const [keepAspect, setKeepAspect] = useState(true);
  const [flags, setFlags] = useState('lanczos');

  const srcW = info?.video?.width || 0;
  const srcH = info?.video?.height || 0;
  const srcRatio = srcW && srcH ? srcW / srcH : 16 / 9;

  useEffect(() => {
    const p = PRESETS.find((x) => x.key === preset);
    if (p && preset !== 'custom') {
      setWidth(p.w);
      setHeight(p.h);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [preset]);

  // 宽高联动用显式事件处理，不用互相 watch ——
  // 双向 watch 会形成回环，浮点误差下可能反复触发
  function onWidthInput(v: number) {
    const w = Number(v) || 0;
    setWidth(w);
    if (keepAspect && w) {
      const h = Math.round(w / srcRatio);
      setHeight(h % 2 === 0 ? h : h - 1);
    }
  }

  function onHeightInput(v: number) {
    const h = Number(v) || 0;
    setHeight(h);
    if (keepAspect && h) {
      const w = Math.round(h * srcRatio);
      setWidth(w % 2 === 0 ? w : w - 1);
    }
  }

  let sizeNote = '';
  if (srcW && width && height) {
    const upscaling = width > srcW;
    const base = `${srcW}×${srcH} → ${width}×${height}`;
    sizeNote = upscaling ? `${base}（目标比源大，画质不会提升）` : base;
  }

  let aspectWarning = '';
  if (keepAspect && srcRatio && width && height) {
    const target = width / height;
    if (Math.abs(target - srcRatio) / srcRatio > 0.02) {
      aspectWarning = '当前宽高比与源画面不一致，画面会被拉伸变形（已关闭锁定比例）';
    }
  }

  function buildSpec() {
    if (!file || !outPath) return null;
    if (!width && !height) return null;
    return {
      kind: 'resize',
      output: outPath,
      ...baseFields(file, info),
      scale: {
        width: keepAspect ? Number(width) : Number(width),
        height: keepAspect ? 0 : Number(height),
        keepAspect: keepAspect,
        flags: flags,
      },
    };
  }

  const { cmd } = useCommandPreview(buildSpec, [file, outPath, width, height, keepAspect, flags]);
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
    const name = (file?.name || 'output').replace(/\.[^.]+$/, '') + '_resized' + ext;
    const p = await api.pickSaveFile(name, `*${ext}`);
    if (p) setOutPath(p);
  }

  // 源本身小于 720p 时，默认按源尺寸，避免无意义的放大。
  // path-keyed：只在新文件的 info 首次到达时跑一次，不跟用户的 preset 选择打架。
  const [lastInitFor, setLastInitFor] = useState('');
  useEffect(() => {
    if (file && info && lastInitFor !== file.path) {
      setLastInitFor(file.path);
      const v = info?.video;
      if (v) {
        const w = v.width;
        if (w && w < 1280) {
          setPreset('custom');
          setWidth(w % 2 === 0 ? w : w - 1);
          setHeight(v.height % 2 === 0 ? v.height : v.height - 1);
        }
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [file, info]);

  return (
    <>
      <div className="card">
        <h2>源文件</h2>
        <FilePicker value={file} onChange={load} label="选择要缩放的视频，也可以直接拖进来" />
        {file && (
          <div style={{ marginTop: 12 }}>
            <MediaInfoCard info={info} loading={loadingInfo} />
            {probeError && (
              <div className="banner warn" style={{ marginTop: 10 }}>
                {probeError}
              </div>
            )}
          </div>
        )}
      </div>

      {file && (
        <div className="card">
          <h2>目标尺寸</h2>

          <ToggleButtonGroup
            selectionMode="single"
            selectedKeys={[preset]}
            onSelectionChange={(keys) => {
              const v = [...keys][0];
              if (v != null) setPreset(String(v));
            }}
          >
            {PRESETS.map((p) => (
              <ToggleButton key={p.key} id={p.key}>
                {p.label}
              </ToggleButton>
            ))}
          </ToggleButtonGroup>

          <div className="grid3" style={{ marginTop: 14 }}>
            <div className="field">
              <label>宽度（像素）</label>
              <AppNumber value={width} min={2} step={2} onChange={onWidthInput} />
            </div>
            <div className="field">
              <label>高度（像素）{keepAspect ? '（自动）' : ''}</label>
              <AppNumber value={height} min={2} step={2} disabled={keepAspect} onChange={onHeightInput} />
            </div>
            <div className="field">
              <label>缩放算法</label>
              <AppSelect value={flags} options={FLAG_OPTIONS} onChange={setFlags} />
            </div>
          </div>

          <div style={{ marginTop: 12 }}>
            <AppCheck checked={keepAspect} onChange={setKeepAspect}>
              锁定宽高比（改一边另一边自动算，且自动取偶数）
            </AppCheck>
          </div>

          {sizeNote && (
            <div className="banner info" style={{ marginTop: 12, marginBottom: 0 }}>
              {sizeNote}
            </div>
          )}
          {aspectWarning && (
            <div className="banner warn" style={{ marginTop: 10, marginBottom: 0 }}>
              {aspectWarning}
            </div>
          )}
        </div>
      )}

      {file && (
        <div className="card">
          <h2>输出</h2>
          <div className="field">
            <label>输出路径</label>
            <div className="row">
              <Input value={outPath} onChange={(e) => setOutPath(e.target.value)} className="mono" style={{ flex: 1 }} />
              <button onClick={pickOutput}>浏览…</button>
            </div>
          </div>

          <CommandPreview steps={cmd.steps} err={cmd.err} />

          <div className="row" style={{ marginTop: 14 }}>
            <button className="primary" disabled={running || !outPath} onClick={run}>
              {running ? '处理中…' : '开始缩放'}
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
