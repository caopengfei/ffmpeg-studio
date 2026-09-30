import { useEffect, useRef, useState, type JSX } from 'react';
import { Accordion, ProgressBar } from '@heroui/react';
import { AnimatePresence, motion } from 'framer-motion';
import { api, events, formatSize } from './api';
import TranscodeTab from './tabs/TranscodeTab';
import CompressTab from './tabs/CompressTab';
import TrimTab from './tabs/TrimTab';
import ResizeTab from './tabs/ResizeTab';
import SnapshotTab from './tabs/SnapshotTab';
import GifTab from './tabs/GifTab';
import WatermarkTab from './tabs/WatermarkTab';

// 侧栏导航。icon 是 24×24 viewBox 下的 path，统一线稿风格（stroke=currentColor），
// 这样选中/悬停时颜色跟着 CSS 走，不用为每个状态准备两套图标
const tabs = [
  {
    key: 'watermark',
    label: '加水印',
    desc: '多水印叠加，实时预览',
    icon: 'M12 3.2c2.6 3 5.4 6.3 5.4 9.3a5.4 5.4 0 0 1-10.8 0c0-3 2.8-6.3 5.4-9.3z',
  },
  {
    key: 'transcode',
    label: '转码',
    desc: '换容器、换编码格式',
    icon: 'M4 8h13m0 0-3-3m3 3-3 3M20 16H7m0 0 3-3m-3 3 3 3',
  },
  {
    key: 'compress',
    label: '压缩体积',
    desc: '按质量或按目标体积压缩',
    icon: 'M12 3.5v9.2m0 0-3.6-3.6M12 12.7l3.6-3.6M6 19.5h12',
  },
  {
    key: 'trim',
    label: '截取片段',
    desc: '从时间轴上剪一段出来',
    icon: 'M6 3.5v17M18 3.5v17M9.6 8.4h4.8v7.2H9.6z',
  },
  {
    key: 'resize',
    label: '缩放分辨率',
    desc: '改画面尺寸',
    icon: 'M15 3h6v6M9 21H3v-6M21 3l-7.5 7.5M3 21l7.5-7.5',
  },
  {
    key: 'snapshot',
    label: '抽帧截图',
    desc: '抽一帧当截图或封面',
    icon: 'M3 9.5a2 2 0 0 1 2-2h1.5l1.3-2.2h6.4L15.5 7.5H19a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2zM12 17a3.4 3.4 0 1 0 0-6.8 3.4 3.4 0 0 0 0 6.8z',
  },
  {
    key: 'gif',
    label: '转 GIF',
    desc: '片段转成动图',
    icon: 'M4 5.5h16v13H4zM10.4 9.8v4.4l3.9-2.2z',
  },
];

// 全部 Tab 已迁移为真实组件。
const tabComponents: Record<string, () => JSX.Element> = {
  watermark: WatermarkTab,
  transcode: TranscodeTab,
  compress: CompressTab,
  trim: TrimTab,
  resize: ResizeTab,
  snapshot: SnapshotTab,
  gif: GifTab,
};

export default function App(): JSX.Element {
  const [current, setCurrent] = useState('watermark');
  const [ffInfo, setFfInfo] = useState<any>(null);
  const [checking, setChecking] = useState(true);
  const [setting, setSetting] = useState(false);
  const [redetecting, setRedetecting] = useState(false);
  const [copied, setCopied] = useState(false);
  const copyTimer = useRef<number | undefined>(undefined);

  const currentTab = tabs.find((t) => t.key === current)!;
  const envOK = !!ffInfo?.available;

  async function refresh() {
    setChecking(true);
    try {
      setFfInfo(await api.getFFmpegInfo());
    } finally {
      setChecking(false);
    }
  }

  async function browseFFmpeg() {
    setSetting(true);
    try {
      const res = await api.pickFFmpegPath();
      if (res) setFfInfo(res);
    } catch (e: any) {
      window.alert('设置失败：' + (e?.message || e));
    } finally {
      setSetting(false);
    }
  }

  async function redetect() {
    // 用独立状态而不是 checking：checking 为真时整张引导卡片会被切走，
    // 点一下按钮界面闪一下反而让人以为出错了
    setRedetecting(true);
    try {
      setFfInfo(await api.detectFFmpeg());
    } finally {
      setRedetecting(false);
    }
  }

  // --- 没找到 ffmpeg 时的引导 ---

  function openDownload() {
    const url = ffInfo?.downloadUrl;
    if (!url) return;
    api.openURL(url).catch((e: any) => window.alert('打开链接失败：' + (e?.message || e)));
  }

  function openProgramDir() {
    const dir = ffInfo?.programDir;
    if (!dir) return;
    api.openDirectory(dir).catch((e: any) => window.alert('打开目录失败：' + (e?.message || e)));
  }

  async function copyProgramDir() {
    const dir = ffInfo?.programDir;
    if (!dir) return;
    try {
      await navigator.clipboard.writeText(dir);
      setCopied(true);
      window.clearTimeout(copyTimer.current);
      copyTimer.current = window.setTimeout(() => setCopied(false), 1600);
    } catch {
      // 剪贴板被禁用时至少让路径可见可手抄
      window.alert('复制失败，路径是：\n' + dir);
    }
  }

  // --- 自动安装 ffmpeg ---

  const [installing, setInstalling] = useState(false);
  const [installPct, setInstallPct] = useState(0);
  const [installMsg, setInstallMsg] = useState('');
  const [installReceived, setInstallReceived] = useState(0);
  const [installTotal, setInstallTotal] = useState(0);
  const [installSpeed, setInstallSpeed] = useState(0);
  const [installError, setInstallError] = useState('');
  const [showManual, setShowManual] = useState(false);
  const [showTried, setShowTried] = useState(false);

  function onInstallEvent(e: any) {
    // phase: downloading | extracting | done | failed
    if (e.phase === 'done') {
      setInstalling(false);
      setInstallPct(100);
      setInstallMsg('');
      if (e.info) setFfInfo(e.info);
      return;
    }
    if (e.phase === 'failed') {
      setInstalling(false);
      setInstallMsg('');
      setInstallError(e.message || '安装失败');
      return;
    }

    setInstallPct(e.percent || 0);
    setInstallMsg(e.message || '');
    setInstallReceived(e.received || 0);
    setInstallTotal(e.total || 0);
    setInstallSpeed(e.speed || 0);
  }

  async function startInstall() {
    setInstallError('');
    setInstallMsg('');
    setInstallPct(0);
    setInstallReceived(0);
    setInstallTotal(0);
    setInstallSpeed(0);
    setInstalling(true);
    try {
      await api.installFFmpeg();
    } catch (e: any) {
      setInstalling(false);
      setInstallError(e?.message || String(e));
    }
  }

  async function cancelInstall() {
    try {
      await api.cancelFFmpegInstall();
    } finally {
      setInstalling(false);
      setInstallMsg('');
    }
  }

  useEffect(() => {
    refresh();
    const off = events.EventsOn('ffmpeg:install', onInstallEvent);
    return () => {
      if (typeof off === 'function') (off as () => void)();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const TabBody = tabComponents[current] ?? WatermarkTab;

  return (
    <div className="shell">
      <aside className="sidebar">
        <div className="brand">
          <span className="brand-mark">
            <svg
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.8"
              strokeLinecap="round"
              strokeLinejoin="round"
              aria-hidden="true"
            >
              <path d="M5 4.5h14v15H5z" />
              <path d="M10.4 9.4v5.2l4.2-2.6z" />
            </svg>
          </span>
          <span className="brand-text">
            <span className="brand-title">FFmpeg Studio</span>
            <small>本地音视频处理</small>
          </span>
        </div>

        <nav className="nav" aria-label="功能导航">
          {tabs.map((t) => (
            <button
              key={t.key}
              className="nav-item"
              data-state={current === t.key ? 'active' : 'inactive'}
              title={t.desc}
              onClick={() => setCurrent(t.key)}
            >
              <span className="nav-icon">
                <svg
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="1.6"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  aria-hidden="true"
                >
                  <path d={t.icon} />
                </svg>
              </span>
              <span className="nav-text">
                <span className="nav-label">{t.label}</span>
                <span className="nav-desc">{t.desc}</span>
              </span>
            </button>
          ))}
        </nav>

        <div className="sidebar-foot">
          <div className={`env-badge${!envOK && !checking ? ' bad' : ''}`}>
            <span className="env-dot"></span>
            <span className="env-text">
              {checking ? (
                '正在检测 ffmpeg…'
              ) : envOK ? (
                <>
                  <b>ffmpeg {ffInfo.version}</b>
                  <span className="env-sub">{ffInfo.source}</span>
                </>
              ) : (
                <>
                  <b>未找到 ffmpeg</b>
                  <span className="env-sub">放到程序同级目录即可</span>
                </>
              )}
            </span>
          </div>
          <button className="ghost tiny" style={{ width: '100%' }} onClick={redetect}>
            重新检测
          </button>
        </div>
      </aside>

      <main className="main">
        {/* 没找到 ffmpeg 时挡在最前面，避免用户白填一堆参数 */}
        {!checking && !envOK ? (
          <div className="card">
            <h2>还没有 ffmpeg</h2>

            {/* 只下了一个 ffmpeg.exe 的情况：单独点破，否则用户会以为程序坏了 */}
            {ffInfo?.incomplete && (
              <div className="banner warn" style={{ marginTop: 12 }}>
                找到了 <code>{ffInfo.incomplete}</code>，但它旁边没有 <code>ffprobe.exe</code>。
                本程序要靠 ffprobe 读时长和分辨率，单个 <code>ffmpeg.exe</code> 用不了 ——
                请下载下面这个<b>完整包</b>。
              </div>
            )}

            <p className="lead">
              转码、压缩、切片都靠 ffmpeg 干活，得先把它装好。
              已经自动找过<b>程序同级目录</b>、常见安装位置和系统 PATH，都没找到。
            </p>

            <ol className="setup-steps">
              <li>
                <span className="step-no">1</span>
                <div className="step-body">
                  <b>让程序自己装好</b>
                  <p className="tip-line">
                    从官方源下载完整包（约 110&nbsp;MB），自动解压到程序目录的 <code>bin</code>{' '}
                    下并立刻启用。 网速慢的话要几分钟，随时可以取消。
                  </p>

                  {/* 安装中：进度 + 取消 */}
                  {installing ? (
                    <div className="install-box">
                      <ProgressBar
                        className="install-bar"
                        value={installPct}
                        minValue={0}
                        maxValue={100}
                        aria-label="安装进度"
                      >
                        <ProgressBar.Track className="install-bar-track">
                          <ProgressBar.Fill className="install-bar-fill" />
                        </ProgressBar.Track>
                      </ProgressBar>
                      <div className="install-meta">
                        <span className="install-msg">{installMsg || '准备中…'}</span>
                        <span className="mono">
                          {installPct.toFixed(0)}%
                          {installReceived ? <>· {formatSize(installReceived)}</> : null}
                          {installSpeed ? <>· {formatSize(installSpeed)}/s</> : null}
                        </span>
                      </div>
                      <button className="tiny" onClick={cancelInstall}>
                        取消
                      </button>
                    </div>
                  ) : (
                    <div className="row">
                      <button className="primary" onClick={startInstall}>
                        自动下载并安装
                      </button>
                      <button onClick={() => setShowManual(!showManual)}>
                        {showManual ? '收起手动步骤' : '我要手动装'}
                      </button>
                    </div>
                  )}

                  {installError && (
                    <div className="banner err" style={{ margin: '10px 0 0' }}>
                      自动安装失败：{installError}
                      {!showManual && (
                        <>
                          <br />
                          网络受限的话可以改用下面的手动步骤。
                        </>
                      )}
                    </div>
                  )}
                </div>
              </li>
            </ol>

            {/* 手动安装三步：折叠收放，默认收起 */}
            <Accordion.Root
              hideSeparator
              expandedKeys={showManual ? ['manual'] : []}
              onExpandedChange={(keys) => setShowManual(keys.has('manual'))}
            >
              <Accordion.Item id="manual">
                <Accordion.Panel className="manual-steps">
                  <div className="step-row">
                    <span className="step-no">2</span>
                    <div className="step-body">
                      <b>自己下载完整版</b>
                      <p className="tip-line">
                        约 110&nbsp;MB。必须是带 <code>ffprobe</code> 的完整包 —— 光有一个{' '}
                        <code>ffmpeg.exe</code> 是没法用的。
                      </p>
                      <div className="row">
                        <button onClick={openDownload}>打开下载页</button>
                      </div>
                    </div>
                  </div>

                  <div className="step-row">
                    <span className="step-no">3</span>
                    <div className="step-body">
                      <b>解压后整个文件夹丢进这里</b>
                      <p className="tip-line">不用手动挑 exe，程序会自己往下找两三层，所以带目录名也没关系。</p>
                      <div className="path-row">
                        <code title={ffInfo?.programDir}>{ffInfo?.programDir || '（没取到程序目录）'}</code>
                        <button className="tiny" disabled={!ffInfo?.programDir} onClick={openProgramDir}>
                          打开
                        </button>
                        <button className="tiny" disabled={!ffInfo?.programDir} onClick={copyProgramDir}>
                          {copied ? '已复制' : '复制'}
                        </button>
                      </div>
                    </div>
                  </div>

                  <div className="step-row">
                    <span className="step-no">4</span>
                    <div className="step-body">
                      <b>回到这里重新检测</b>
                      <p className="tip-line">放好之后点一下，几秒内就能认出来。</p>
                      <div className="row">
                        <button className="primary" disabled={redetecting} onClick={redetect}>
                          {redetecting ? '检测中…' : '重新检测'}
                        </button>
                        <button disabled={setting} onClick={browseFFmpeg}>
                          {setting ? '校验中…' : '我已经有了，手动选择'}
                        </button>
                      </div>
                    </div>
                  </div>
                </Accordion.Panel>
              </Accordion.Item>
            </Accordion.Root>

            <Accordion.Root
              className="probe-detail"
              hideSeparator
              expandedKeys={showTried ? ['tried'] : []}
              onExpandedChange={(keys) => setShowTried(keys.has('tried'))}
            >
              <Accordion.Item id="tried">
                <Accordion.Heading>
                  <Accordion.Trigger className="probe-summary">
                    <span className={`caret${showTried ? ' open' : ''}`}>›</span>
                    照做了还是认不出来？
                  </Accordion.Trigger>
                </Accordion.Heading>
                <Accordion.Panel>
                  <p className="tip-line">这次一共找过这些位置（顺序即优先级）：</p>
                  <ul>
                    {(ffInfo?.tried || []).map((p: string, i: number) => (
                      <li key={i}>
                        <code>{p}</code>
                      </li>
                    ))}
                  </ul>
                </Accordion.Panel>
              </Accordion.Item>
            </Accordion.Root>
          </div>
        ) : (
          <AnimatePresence mode="wait">
            <motion.div
              key={current}
              className="page"
              initial={{ opacity: 0, y: 12 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -8 }}
              transition={{ duration: 0.16, ease: 'easeOut' }}
            >
              <div className="page-head">
                <h1>{currentTab.label}</h1>
                <p>{currentTab.desc}</p>
              </div>

              <TabBody />
            </motion.div>
          </AnimatePresence>
        )}
      </main>
    </div>
  );
}
