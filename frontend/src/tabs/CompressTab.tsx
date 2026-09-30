import { useState } from 'react';
import { Input, ToggleButton, ToggleButtonGroup } from '@heroui/react';
import FilePicker from '../components/FilePicker';
import MediaInfoCard from '../components/MediaInfoCard';
import CommandPreview from '../components/CommandPreview';
import ProgressPanel from '../components/ProgressPanel';
import { api, formatSize } from '../api';
import AppSelect from '../ui/AppSelect';
import AppSlider from '../ui/AppSlider';
import AppNumber from '../ui/AppNumber';
import AppCheck from '../ui/AppCheck';
import { useTask } from '../hooks/useTask';
import { useMediaSource } from '../hooks/useMediaSource';
import { useCommandPreview } from '../hooks/useCommandPreview';
import { baseFields } from '../hooks/baseFields';

const PRESET_OPTIONS = [
  { value: 'veryfast', label: 'veryfast' },
  { value: 'fast', label: 'fast' },
  { value: 'medium', label: 'medium（默认）' },
  { value: 'slow', label: 'slow' },
  { value: 'veryslow', label: 'veryslow' },
];
const AUDIO_KBPS_OPTS = [
  { value: 64, label: '64 · 语音' },
  { value: 96, label: '96' },
  { value: 128, label: '128 · 标准' },
  { value: 192, label: '192 · 高保真' },
];

export default function CompressTab() {
  const { file, info, loadingInfo, probeError, outPath, setOutPath, load } = useMediaSource('compress');

  const [mode, setMode] = useState('quality'); // quality | targetSize
  const [crf, setCrf] = useState(26);
  const [preset, setPreset] = useState('medium');
  const [copyAudio, setCopyAudio] = useState(true);

  const [targetMB, setTargetMB] = useState(0);
  const [audioKbps, setAudioKbps] = useState(128);

  // 目标体积模式下，先给用户看清楚码率是怎么算出来的
  let computedBitrate: { totalKbps: number; videoKbps: number } | null = null;
  {
    const dur = info?.duration || 0;
    if (dur && targetMB) {
      const totalKbps = (targetMB * 8192) / dur;
      const videoKbps = totalKbps - Number(audioKbps);
      computedBitrate = { totalKbps, videoKbps };
    }
  }

  let sizeWarning = '';
  if (computedBitrate) {
    if (computedBitrate.videoKbps < 100) {
      sizeWarning = '目标体积太小，视频码率已低于可用下限，请调大目标或降低音频码率';
    } else {
      const origin = info?.size || 0;
      if (origin && targetMB * 1048576 > origin) {
        sizeWarning = '目标体积比源文件还大，压出来反而会变大';
      }
    }
  }

  function buildSpec() {
    if (!file || !outPath) return null;
    if (mode === 'quality') {
      return {
        kind: 'compress',
        output: outPath,
        ...baseFields(file, info),
        compress: { mode: 'quality', crf: Number(crf), preset: preset, copyAudio: copyAudio },
      };
    }
    if (!targetMB) return null;
    return {
      kind: 'compress',
      output: outPath,
      ...baseFields(file, info),
      compress: { mode: 'targetSize', targetMb: Number(targetMB), audioKbps: Number(audioKbps) },
    };
  }

  const { cmd } = useCommandPreview(buildSpec, [file, outPath, mode, crf, preset, copyAudio, targetMB, audioKbps, info]);
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
    const name = (file?.name || 'output').replace(/\.[^.]+$/, '') + '_compressed.mp4';
    const p = await api.pickSaveFile(name, '*.mp4');
    if (p) setOutPath(p);
  }

  function initTarget() {
    if (targetMB) return;
    const s = info?.size || 0;
    if (s) setTargetMB(Math.max(1, Math.round((s / 1048576) * 0.4 * 10) / 10));
  }

  return (
    <>
      <div className="card">
        <h2>源文件</h2>
        <FilePicker value={file} onChange={load} label="选择要压缩的视频，也可以直接拖进来" />
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
          <h2>压缩方式</h2>

          <div className="modes">
            <button className={`mode${mode === 'quality' ? ' on' : ''}`} onClick={() => setMode('quality')}>
              <b>按质量</b>
              <span>调画质档位，体积不固定。速度快</span>
            </button>
            <button
              className={`mode${mode === 'targetSize' ? ' on' : ''}`}
              onClick={() => {
                setMode('targetSize');
                initTarget();
              }}
            >
              <b>按目标体积</b>
              <span>压缩到指定大小，走两遍编码。耗时约翻倍</span>
            </button>
          </div>

          {mode === 'quality' ? (
            <>
              <div className="grid2" style={{ marginTop: 14 }}>
                <div className="field">
                  <label>画质 CRF：{crf}</label>
                  <AppSlider value={crf} min={18} max={32} step={1} label="画质 CRF" onChange={setCrf} />
                  <span className="tip">越小越清晰。23 默认，28 明显变小，32 以上能看出损伤</span>
                </div>
                <div className="field">
                  <label>编码速度</label>
                  <AppSelect value={preset} options={PRESET_OPTIONS} onChange={setPreset} />
                </div>
              </div>
              <div style={{ marginTop: 12 }}>
                <AppCheck checked={copyAudio} onChange={setCopyAudio}>
                  音频直接复制，不重新编码（更快且无损）
                </AppCheck>
              </div>
            </>
          ) : (
            <>
              <div className="grid3" style={{ marginTop: 14 }}>
                <div className="field">
                  <label>目标体积（MB）</label>
                  <AppNumber value={targetMB} min={0.1} step={0.1} label="目标体积" onChange={setTargetMB} />
                  {info && <span className="tip">源文件 {formatSize(info.size)}</span>}
                </div>
                <div className="field">
                  <label>音频码率（kbps）</label>
                  <ToggleButtonGroup
                    selectionMode="single"
                    selectedKeys={[String(audioKbps)]}
                    onSelectionChange={(keys) => {
                      const v = [...keys][0];
                      if (v != null) setAudioKbps(Number(v));
                    }}
                  >
                    {AUDIO_KBPS_OPTS.map((o) => (
                      <ToggleButton key={o.value} id={String(o.value)}>
                        {o.label}
                      </ToggleButton>
                    ))}
                  </ToggleButtonGroup>
                </div>
                <div className="field">
                  <label>计算出的视频码率</label>
                  <div className="calc mono">
                    {computedBitrate ? Math.round(computedBitrate.videoKbps) + ' kbps' : '—'}
                  </div>
                  {computedBitrate && (
                    <span className="tip">
                      总码率 {Math.round(computedBitrate.totalKbps)} kbps − 音频 {audioKbps}
                    </span>
                  )}
                </div>
              </div>

              {sizeWarning ? (
                <div className="banner warn" style={{ marginTop: 12, marginBottom: 0 }}>
                  {sizeWarning}
                </div>
              ) : (
                <div className="banner info" style={{ marginTop: 12, marginBottom: 0 }}>
                  两遍编码：第一遍分析画面，第二遍正式编码。总耗时约为单遍的两倍，但体积能贴近目标值。
                </div>
              )}
            </>
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
            <button className="primary" disabled={running || !outPath || !!sizeWarning} onClick={run}>
              {running ? '处理中…' : '开始压缩'}
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
