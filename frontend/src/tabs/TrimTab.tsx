import { useEffect, useRef, useState } from 'react';
import { Input } from '@heroui/react';
import FilePicker from '../components/FilePicker';
import MediaInfoCard from '../components/MediaInfoCard';
import VideoScrubber from '../components/VideoScrubber';
import CommandPreview from '../components/CommandPreview';
import ProgressPanel from '../components/ProgressPanel';
import { api } from '../api';
import { useTask } from '../hooks/useTask';
import { useMediaSource } from '../hooks/useMediaSource';
import { useCommandPreview } from '../hooks/useCommandPreview';
import { baseFields } from '../hooks/baseFields';

export default function TrimTab() {
  const { file, info, loadingInfo, probeError, outPath, setOutPath, previewURL, previewNote, preparing, load } =
    useMediaSource('trim', { withPreview: true });

  const [accurate, setAccurate] = useState(false);
  const [range, setRange] = useState({ start: 0, end: 0 });

  // load 是异步的：await load(f) 之后读 info 拿到的是闭包里的旧值（React state
  // 不会同步更新，Vue ref 会）。所以用文件指纹（path + duration + size）守卫的
  // useEffect，等 info 到位后给新文件初始化一次 range，不跟用户后续的手动调整打架。
  const initKey = useRef('');
  useEffect(() => {
    if (!file || !info) return;
    const key = `${file.path}::${info.duration ?? ''}::${info.size ?? ''}`;
    if (initKey.current === key) return;
    initKey.current = key;
    setRange({ start: 0, end: info.duration || 0 });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [file, info]);

  const segLen = Math.max(0, (range.end || 0) - (range.start || 0));

  function buildSpec() {
    if (!file || !outPath || !info) return null;
    if (!range.end) return null;
    return {
      kind: 'trim',
      output: outPath,
      ...baseFields(file, info),
      trim: { start: Number(range.start), end: Number(range.end), accurate: accurate },
    };
  }

  const { cmd } = useCommandPreview(buildSpec, [file, outPath, accurate, range, info]);
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
    const name = (file?.name || 'output').replace(/\.[^.]+$/, '') + '_trimmed' + ext;
    const p = await api.pickSaveFile(name, `*${ext}`);
    if (p) setOutPath(p);
  }

  const modeHint = accurate
    ? '精确模式：逐帧定位，起点终点都准。速度取决于片段长度'
    : '快速模式：不重新编码，秒级完成。但起止点会吸附到最近的关键帧，可能差 1~2 秒';

  return (
    <>
      <div className="card">
        <h2>源文件</h2>
        <FilePicker value={file} onChange={load} label="选择要截取的视频，也可以直接拖进来" />
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
            选择片段 <span className="hint">拖动两端手柄调整范围，点上任意位置可跳到该处预览</span>
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
          <h2>截取方式</h2>
          <div className="modes">
            <button className={`mode${!accurate ? ' on' : ''}`} onClick={() => setAccurate(false)}>
              <b>快速</b>
              <span>不重编码，秒级完成</span>
            </button>
            <button className={`mode${accurate ? ' on' : ''}`} onClick={() => setAccurate(true)}>
              <b>精确</b>
              <span>逐帧精确，需要重新编码</span>
            </button>
          </div>
          <div className="banner info" style={{ marginTop: 12, marginBottom: 0 }}>
            {modeHint}
          </div>
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
              {running ? '处理中…' : `开始截取（${segLen.toFixed(1)} 秒）`}
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
