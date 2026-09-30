import { useRef, useState } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import FilePicker from '../components/FilePicker';
import MediaInfoCard from '../components/MediaInfoCard';
import CommandPreview from '../components/CommandPreview';
import ProgressPanel from '../components/ProgressPanel';
import { api } from '../api';
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
import { clamp, timeSummary } from '../lib/watermarkTime';

export default function WatermarkTab() {
  const { file, info, loadingInfo, probeError, outPath, setOutPath, previewURL, previewNote, preparing, load } =
    useMediaSource('watermark', { withPreview: true });

  const [items, setItems] = useState<any[]>([]);
  const [selectedId, setSelectedId] = useState('');
  const [showSafe, setShowSafe] = useState(false);
  const [targetMode, setTargetMode] = useState('source'); // source | 720 | 1080
  const seq = useRef(0);

  // 所有水印字段变更都走这里：React 状态不可变，禁止直接改 items[i].x
  function patchItem(id: string, patch: Record<string, any>) {
    setItems((prev) => prev.map((it) => (it.id === id ? { ...it, ...patch } : it)));
  }

  function replaceItem(id: string, fn: (it: any) => any) {
    setItems((prev) => prev.map((it) => (it.id === id ? fn(it) : it)));
  }

  const selected = items.find((i) => i.id === selectedId) || null;
  const activeCount = items.filter((i) => i.enabled).length;

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

  function summaryOf(it: any) {
    return timeSummary(it, duration);
  }

  // Task 11 接管：调完时间段后自动把播放头挪到该水印可见的时刻。
  // 这里还没有 videoEl，先做 no-op，只改起止数值。
  function afterTimeEdit(_it: any) {
    // Task 11 接管
  }

  function clearTime(it: any) {
    patchItem(it.id, { hasTime: false, start: 0, end: 0 });
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
    const next = items.filter((i) => i.id !== id);
    setItems(next);
    if (selectedId === id) setSelectedId(next[0]?.id || '');
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

  const { cmd } = useCommandPreview(buildSpec, [file, outPath, items, targetMode]);
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
          {/* 预览区：画布拖拽 + 时间轴由 Task 11 接管 */}
          <div>
            <div className="card">预览画布（Task 11）</div>
            <div className="card">时间轴（Task 11）</div>
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
                          <AppSwitch checked={!!it.enabled} onChange={(v) => patchItem(it.id, { enabled: v })} />
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
                        <AppColor value={selected.color || '#ffffff'} onChange={(v) => patchItem(selected.id, { color: v })} />
                      </div>
                      <div className="field">
                        <label>描边宽度</label>
                        <AppNumber value={selected.borderW} min={0} max={10} step={1} onChange={(v) => patchItem(selected.id, { borderW: v })} />
                      </div>
                    </div>
                    <div className="grid2" style={{ marginTop: 10 }}>
                      <div className="field">
                        <label>描边颜色</label>
                        <AppColor value={selected.borderColor || '#000000'} onChange={(v) => patchItem(selected.id, { borderColor: v })} />
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
