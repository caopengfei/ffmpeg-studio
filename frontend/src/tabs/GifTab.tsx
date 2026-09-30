import { useEffect, useRef, useState } from 'react';
import { Input } from '@heroui/react';
import FilePicker from '../components/FilePicker';
import MediaInfoCard from '../components/MediaInfoCard';
import VideoScrubber from '../components/VideoScrubber';
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

const LOOP_OPTIONS = [
  { value: '-1', label: '无限循环' },
  { value: '1', label: '播放 1 次' },
  { value: '3', label: '播放 3 次' },
  { value: '5', label: '播放 5 次' },
];

export default function GifTab() {
  const { file, info, loadingInfo, probeError, outPath, setOutPath, previewURL, previewNote, preparing, load } =
    useMediaSource('gif', { withPreview: true });

  const [range, setRange] = useState({ start: 0, end: 0 });
  const [fps, setFps] = useState(12);
  const [width, setWidth] = useState(480);
  const [twoPass, setTwoPass] = useState(true);
  // AppSelect 的值是字符串，生成参数时再 Number() 回来
  const [loop, setLoop] = useState('-1');

  const initKey = useRef('');
  useEffect(() => {
    if (!file || !info) return;
    const key = `${file.path}::${info.duration ?? ''}::${info.size ?? ''}`;
    if (initKey.current === key) return;
    initKey.current = key;
    const dur = info.duration || 0;
    // GIF 体积涨得很快，默认只取前 6 秒
    setRange({ start: 0, end: Math.min(dur, 6) });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [file, info]);

  const segLen = Math.max(0, (range.end || 0) - (range.start || 0));
  const frames = Math.round(segLen * fps);

  // 粗估体积：GIF 帧数与画面尺寸是主要因素，这里给量级参考
  let estimate = '';
  {
    const px = (Number(width) || 0) * (Number(width) || 0) * 0.5625;
    if (px && frames) {
      const bytes = frames * px * 0.14;
      estimate = formatSize(bytes);
    }
  }

  const tooLong = segLen > 15;

  function buildSpec() {
    if (!file || !outPath || !info) return null;
    if (!range.end) return null;
    return {
      kind: 'gif',
      output: outPath,
      ...baseFields(file, info),
      gif: {
        start: Number(range.start),
        end: Number(range.end),
        fps: Number(fps),
        width: Number(width),
        twoPass: twoPass,
        loop: Number(loop),
      },
    };
  }

  const { cmd } = useCommandPreview(buildSpec, [file, outPath, range, fps, width, twoPass, loop, info]);
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
    const name = (file?.name || 'output').replace(/\.[^.]+$/, '') + '_clip.gif';
    const p = await api.pickSaveFile(name, '*.gif');
    if (p) setOutPath(p);
  }

  // 结果直接内嵌播放，所见即所得
  const [gifURL, setGifURL] = useState('');
  useEffect(() => {
    const r: any = result;
    if (r?.ok && r.outputPath?.toLowerCase().endsWith('.gif')) {
      api.mediaURL(r.outputPath).then((u) => setGifURL(u));
    } else {
      setGifURL('');
    }
  }, [result]);

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
        <div className="card">
          <h2>
            选择片段 <span className="hint">GIF 体积增长很快，建议不超过 10 秒</span>
          </h2>
          <VideoScrubber
            src={previewURL}
            duration={info?.duration || 0}
            mode="range"
            value={range}
            onChange={(v) => setRange({ start: v.start, end: v.end })}
          />
        </div>
      )}

      {file && (
        <div className="card">
          <h2>GIF 参数</h2>

          <div className="grid3">
            <div className="field">
              <label>帧率：{fps} fps</label>
              <AppSlider value={fps} min={5} max={30} step={1} label="帧率" onChange={setFps} />
              <span className="tip">越高越流畅，体积也越大</span>
            </div>
            <div className="field">
              <label>宽度（像素）</label>
              <AppNumber value={width} min={60} max={1280} step={10} label="宽度" onChange={setWidth} />
              <span className="tip">高度按画面比例自动</span>
            </div>
            <div className="field">
              <label>循环</label>
              <AppSelect value={loop} options={LOOP_OPTIONS} onChange={setLoop} />
            </div>
          </div>

          <div style={{ marginTop: 12 }}>
            <AppCheck checked={twoPass} onChange={setTwoPass}>
              高质量模式（先生成专属调色板再编码，画质差别很大，耗时约两倍）
            </AppCheck>
          </div>

          <div className="stats">
            <span>
              片段 <b>{segLen.toFixed(1)}</b> 秒
            </span>
            <span>
              约 <b>{frames}</b> 帧
            </span>
            {estimate && (
              <span>
                预估体积 <b>{estimate}</b>
              </span>
            )}
          </div>

          {tooLong && (
            <div className="banner warn" style={{ marginTop: 12, marginBottom: 0 }}>
              片段偏长，GIF 可能会很大。建议缩短范围或降低帧率、宽度。
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
            <button className="primary" disabled={running || !outPath || segLen <= 0} onClick={run}>
              {running ? '处理中…' : '开始生成 GIF'}
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

      {gifURL && (
        <div className="card">
          <h2>生成结果</h2>
          <img src={gifURL} alt="生成的 GIF" className="gif-out" />
        </div>
      )}
    </>
  );
}
