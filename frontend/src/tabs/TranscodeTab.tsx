import { useEffect, useState } from 'react';
import { Input } from '@heroui/react';
import FilePicker from '../components/FilePicker';
import MediaInfoCard from '../components/MediaInfoCard';
import CommandPreview from '../components/CommandPreview';
import ProgressPanel from '../components/ProgressPanel';
import { api } from '../api';
import AppSelect from '../ui/AppSelect';
import AppSlider from '../ui/AppSlider';
import { useTask } from '../hooks/useTask';
import { useMediaSource } from '../hooks/useMediaSource';
import { useCommandPreview } from '../hooks/useCommandPreview';
import { baseFields } from '../hooks/baseFields';

const CONTAINER_OPTIONS = [
  { value: 'mp4', label: 'MP4 (.mp4)' },
  { value: 'mkv', label: 'MKV (.mkv)' },
  { value: 'mov', label: 'MOV (.mov)' },
  { value: 'webm', label: 'WebM (.webm)' },
  { value: 'mp3', label: '仅音频 · MP3' },
  { value: 'm4a', label: '仅音频 · M4A' },
  { value: 'wav', label: '仅音频 · WAV' },
];
const PRESET_OPTIONS = [
  { value: 'ultrafast', label: 'ultrafast（最快）' },
  { value: 'veryfast', label: 'veryfast' },
  { value: 'fast', label: 'fast' },
  { value: 'medium', label: 'medium（默认）' },
  { value: 'slow', label: 'slow' },
  { value: 'veryslow', label: 'veryslow（最小体积）' },
];

const AUDIO_ONLY = ['mp3', 'm4a', 'wav'];

const VIDEO_CODECS: Record<string, string[]> = {
  mp4: ['libx264', 'libx265', 'libsvtav1', 'h264_qsv', 'hevc_qsv', 'copy'],
  mkv: ['libx264', 'libx265', 'libsvtav1', 'libvpx-vp9', 'h264_qsv', 'hevc_qsv', 'copy'],
  mov: ['libx264', 'libx265', 'prores_ks', 'copy'],
  webm: ['libvpx-vp9', 'libsvtav1', 'copy'],
};
const AUDIO_CODECS: Record<string, string[]> = {
  mp4: ['aac', 'libmp3lame', 'copy'],
  mkv: ['aac', 'libopus', 'libmp3lame', 'copy'],
  mov: ['aac', 'copy'],
  webm: ['libopus', 'copy'],
  mp3: ['libmp3lame'],
  m4a: ['aac'],
  wav: ['pcm_s16le'],
};

export default function TranscodeTab() {
  const { file, info, loadingInfo, probeError, outPath, setOutPath, load } = useMediaSource('transcode');

  const [container, setContainer] = useState('mp4');
  const [vcodec, setVcodec] = useState('libx264');
  const [acodec, setAcodec] = useState('aac');
  const [crf, setCrf] = useState(23);
  const [preset, setPreset] = useState('medium');

  const isAudioOnly = AUDIO_ONLY.includes(container);

  const videoChoices = VIDEO_CODECS[container] || VIDEO_CODECS.mp4;
  const audioChoices = AUDIO_CODECS[container] || AUDIO_CODECS.mp4;
  const videoOpts = videoChoices.map((c) => ({ value: c, label: c }));
  const audioOpts = audioChoices.map((c) => ({ value: c, label: c }));

  // 换容器时把不再支持的编码器纠正回来，避免出现无效组合
  useEffect(() => {
    if (!isAudioOnly && !videoChoices.includes(vcodec)) {
      setVcodec(videoChoices[0]);
    }
    if (!audioChoices.includes(acodec)) {
      setAcodec(audioChoices[0]);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [container]);

  const losslessVideo = vcodec === 'copy' || isAudioOnly;

  function buildSpec() {
    if (!file || !outPath) return null;
    return {
      kind: 'transcode',
      output: outPath,
      ...baseFields(file, info),
      video: isAudioOnly
        ? { codec: 'none' }
        : {
            codec: vcodec,
            crf: losslessVideo ? 0 : Number(crf),
            preset: losslessVideo ? '' : preset,
            pixFmt: losslessVideo ? '' : 'yuv420p',
          },
      audio: { codec: acodec },
    };
  }

  const { cmd } = useCommandPreview(buildSpec, [file, outPath, container, vcodec, acodec, crf, preset]);

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
    const ext = '.' + container;
    const name = (file?.name || 'output').replace(/\.[^.]+$/, '') + '_transcoded' + ext;
    const p = await api.pickSaveFile(name, `*${ext}`);
    if (p) setOutPath(p);
  }

  let hint = '';
  if (vcodec === 'copy') hint = '视频直接复制，不重新编码 —— 速度极快但需要目标容器支持原编码格式';
  else if (container === 'webm') hint = 'WebM 只支持 VP8/VP9/AV1 视频与 Opus/Vorbis 音频';

  return (
    <>
      <div className="card">
        <h2>源文件</h2>
        <FilePicker value={file} onChange={load} label="选择视频或音频文件，也可以直接拖进来" />
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
          <h2>转码设置</h2>

          <div className="grid3">
            <div className="field">
              <label>输出容器</label>
              <AppSelect value={container} options={CONTAINER_OPTIONS} onChange={setContainer} />
            </div>

            {!isAudioOnly && (
              <div className="field">
                <label>视频编码器</label>
                <AppSelect value={vcodec} options={videoOpts} onChange={setVcodec} />
              </div>
            )}

            <div className="field">
              <label>音频编码器</label>
              <AppSelect value={acodec} options={audioOpts} onChange={setAcodec} />
            </div>
          </div>

          {!losslessVideo && (
            <div className="grid2" style={{ marginTop: 12 }}>
              <div className="field">
                <label>画质 CRF：{crf}</label>
                <AppSlider value={crf} min={18} max={32} step={1} onChange={setCrf} />
                <span className="tip">数值越小画质越好、文件越大。18 接近无损，23 是默认，28 以上明显变小</span>
              </div>
              <div className="field">
                <label>编码速度</label>
                <AppSelect value={preset} options={PRESET_OPTIONS} onChange={setPreset} />
                <span className="tip">越慢压得越小，但耗时成倍增加</span>
              </div>
            </div>
          )}

          {hint && (
            <div className="banner info" style={{ marginTop: 12, marginBottom: 0 }}>
              {hint}
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
              {running ? '处理中…' : '开始转码'}
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
