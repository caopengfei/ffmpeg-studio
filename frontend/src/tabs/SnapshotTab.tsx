import { useEffect, useRef, useState } from 'react';
import { Input } from '@heroui/react';
import FilePicker from '../components/FilePicker';
import MediaInfoCard from '../components/MediaInfoCard';
import VideoScrubber from '../components/VideoScrubber';
import CommandPreview from '../components/CommandPreview';
import ProgressPanel from '../components/ProgressPanel';
import { api, formatTime } from '../api';
import AppSelect from '../ui/AppSelect';
import AppNumber from '../ui/AppNumber';
import AppCheck from '../ui/AppCheck';
import { useTask } from '../hooks/useTask';
import { useMediaSource } from '../hooks/useMediaSource';
import { useCommandPreview } from '../hooks/useCommandPreview';
import { baseFields } from '../hooks/baseFields';

const FORMAT_OPTIONS = [
  { value: 'png', label: 'PNG（无损）' },
  { value: 'jpg', label: 'JPG（体积小）' },
  { value: 'webp', label: 'WebP' },
];

export default function SnapshotTab() {
  const { file, info, loadingInfo, probeError, outPath, setOutPath, previewURL, previewNote, preparing, load } =
    useMediaSource('snapshot', { withPreview: true });

  const [point, setPoint] = useState({ time: 0 });
  const [format, setFormat] = useState('png');
  const [quality, setQuality] = useState(2);

  const [batch, setBatch] = useState(false);
  const [batchEvery, setBatchEvery] = useState(5);
  const [outDir, setOutDir] = useState('');

  const [asCover, setAsCover] = useState(false);
  const [coverImage, setCoverImage] = useState<any>(null);

  const initKey = useRef('');
  useEffect(() => {
    if (!file || !info) return;
    const key = `${file.path}::${info.duration ?? ''}::${info.size ?? ''}`;
    if (initKey.current === key) return;
    initKey.current = key;
    setPoint({ time: Math.min(1, (info.duration || 2) * 0.1) });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [file, info]);

  const qualityLabel = format === 'jpg' ? '质量（1 最好 / 31 最差）' : '质量（1–100）';

  function buildSpec() {
    if (!file || !info) return null;

    if (batch) {
      if (!outDir) return null;
      return {
        kind: 'snapshot',
        output: '',
        ...baseFields(file, info),
        snapshot: { format: format, quality: Number(quality), batchEvery: Number(batchEvery), outDir: outDir },
      };
    }

    if (asCover) {
      if (!coverImage) return null;
      return {
        kind: 'snapshot',
        output: outPath,
        ...baseFields(file, info),
        snapshot: { asCover: true, coverImage: coverImage.path, format: format },
      };
    }

    if (!outPath) return null;
    return {
      kind: 'snapshot',
      output: outPath,
      ...baseFields(file, info),
      snapshot: { time: Number(point.time || 0), format: format, quality: Number(quality) },
    };
  }

  const { cmd } = useCommandPreview(buildSpec, [file, outPath, point, format, quality, batch, batchEvery, outDir, asCover, coverImage, info]);
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
    const ext = format === 'jpg' ? '.jpg' : format === 'webp' ? '.webp' : '.png';
    const name = (file?.name || 'output').replace(/\.[^.]+$/, '') + '_frame' + ext;
    const p = await api.pickSaveFile(name, `*${ext}`);
    if (p) setOutPath(p);
  }

  async function chooseOutDir() {
    const d = await api.pickDirectory('选择保存帧序列的目录');
    if (d) setOutDir(d);
  }

  async function chooseCover() {
    const f = await api.pickImageFile();
    if (f) setCoverImage(f);
  }

  const canRun = !!(batch ? outDir : asCover ? coverImage : outPath);

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

      {file && previewURL && !batch && (
        <div className="card">
          <h2>
            选择时间点 <span className="hint">拖动标记或点轨道，右侧会实时定位到该帧</span>
          </h2>
          <VideoScrubber
            src={previewURL}
            duration={info?.duration || 0}
            mode="point"
            value={point}
            onChange={(v) => setPoint({ time: v.time ?? 0 })}
          />
        </div>
      )}

      {file && (
        <div className="card">
          <h2>抽帧设置</h2>

          <div className="grid3">
            <div className="field">
              <label>输出格式</label>
              <AppSelect value={format} options={FORMAT_OPTIONS} onChange={setFormat} />
            </div>
            {format !== 'png' && (
              <div className="field">
                <label>{qualityLabel}</label>
                <AppNumber value={quality} min={1} max={format === 'jpg' ? 31 : 100} step={1} label={qualityLabel} onChange={setQuality} />
              </div>
            )}
          </div>

          <div className="divider"></div>

          <AppCheck checked={batch} onChange={setBatch}>
            批量抽帧：每隔一段时间抽一帧，导出成图片序列
          </AppCheck>

          {batch && (
            <div className="grid2" style={{ marginTop: 10 }}>
              <div className="field">
                <label>间隔（秒）</label>
                <AppNumber value={batchEvery} min={0.1} step={0.5} label="间隔" onChange={setBatchEvery} />
              </div>
              <div className="field">
                <label>输出目录</label>
                <div className="row">
                  <input value={outDir} type="text" className="mono" style={{ flex: 1 }} readOnly />
                  <button onClick={chooseOutDir}>选择…</button>
                </div>
              </div>
            </div>
          )}

          {!batch && (
            <>
              <div className="divider"></div>
              <AppCheck checked={asCover} onChange={setAsCover}>
                把这帧设为视频封面（写入 attached_pic 流，视频不重编码）
              </AppCheck>
              {asCover && (
                <div className="row" style={{ marginTop: 10 }}>
                  <button onClick={chooseCover}>选择封面图片</button>
                  <span className="mono" style={{ fontSize: 12 }}>
                    {coverImage?.name || '尚未选择'}
                  </span>
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
            <button className="primary" disabled={running || !canRun} onClick={run}>
              {running ? '处理中…' : batch ? '开始批量抽帧' : asCover ? '写入封面' : `抽取 ${formatTime(point.time)} 处的一帧`}
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
