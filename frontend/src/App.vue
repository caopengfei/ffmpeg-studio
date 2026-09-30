<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { TabsRoot, TabsList, TabsTrigger, CollapsibleRoot, CollapsibleTrigger, CollapsibleContent, ProgressRoot, ProgressIndicator } from 'reka-ui'
import { motion, AnimatePresence } from 'motion-v'
import { api, events, formatSize } from './api'

import TranscodeTab from './tabs/TranscodeTab.vue'
import CompressTab from './tabs/CompressTab.vue'
import TrimTab from './tabs/TrimTab.vue'
import ResizeTab from './tabs/ResizeTab.vue'
import SnapshotTab from './tabs/SnapshotTab.vue'
import GifTab from './tabs/GifTab.vue'
import WatermarkTab from './tabs/WatermarkTab.vue'

// 侧栏导航。icon 是 24×24 viewBox 下的 path，统一线稿风格（stroke=currentColor），
// 这样选中/悬停时颜色跟着 CSS 走，不用为每个状态准备两套图标
const tabs = [
  {
    key: 'watermark',
    label: '加水印',
    comp: WatermarkTab,
    desc: '多水印叠加，实时预览',
    icon: 'M12 3.2c2.6 3 5.4 6.3 5.4 9.3a5.4 5.4 0 0 1-10.8 0c0-3 2.8-6.3 5.4-9.3z',
  },
  {
    key: 'transcode',
    label: '转码',
    comp: TranscodeTab,
    desc: '换容器、换编码格式',
    icon: 'M4 8h13m0 0-3-3m3 3-3 3M20 16H7m0 0 3-3m-3 3 3 3',
  },
  {
    key: 'compress',
    label: '压缩体积',
    comp: CompressTab,
    desc: '按质量或按目标体积压缩',
    icon: 'M12 3.5v9.2m0 0-3.6-3.6M12 12.7l3.6-3.6M6 19.5h12',
  },
  {
    key: 'trim',
    label: '截取片段',
    comp: TrimTab,
    desc: '从时间轴上剪一段出来',
    icon: 'M6 3.5v17M18 3.5v17M9.6 8.4h4.8v7.2H9.6z',
  },
  {
    key: 'resize',
    label: '缩放分辨率',
    comp: ResizeTab,
    desc: '改画面尺寸',
    icon: 'M15 3h6v6M9 21H3v-6M21 3l-7.5 7.5M3 21l7.5-7.5',
  },
  {
    key: 'snapshot',
    label: '抽帧截图',
    comp: SnapshotTab,
    desc: '抽一帧当截图或封面',
    icon: 'M3 9.5a2 2 0 0 1 2-2h1.5l1.3-2.2h6.4L15.5 7.5H19a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2zM12 17a3.4 3.4 0 1 0 0-6.8 3.4 3.4 0 0 0 0 6.8z',
  },
  {
    key: 'gif',
    label: '转 GIF',
    comp: GifTab,
    desc: '片段转成动图',
    icon: 'M4 5.5h16v13H4zM10.4 9.8v4.4l3.9-2.2z',
  },
]

const current = ref('watermark')
const ffInfo = ref(null)
const checking = ref(true)
const setting = ref(false)
const redetecting = ref(false)
const copied = ref(false)
let copyTimer = null

const currentTab = computed(() => tabs.find((t) => t.key === current.value))
const envOK = computed(() => !!ffInfo.value?.available)

async function refresh() {
  checking.value = true
  try {
    ffInfo.value = await api.getFFmpegInfo()
  } finally {
    checking.value = false
  }
}

async function browseFFmpeg() {
  setting.value = true
  try {
    const res = await api.pickFFmpegPath()
    if (res) ffInfo.value = res
  } catch (e) {
    window.alert('设置失败：' + (e?.message || e))
  } finally {
    setting.value = false
  }
}

async function redetect() {
  // 用独立状态而不是 checking：checking 为真时整张引导卡片会被切走，
  // 点一下按钮界面闪一下反而让人以为出错了
  redetecting.value = true
  try {
    ffInfo.value = await api.detectFFmpeg()
  } finally {
    redetecting.value = false
  }
}

// --- 没找到 ffmpeg 时的引导 ---

function openDownload() {
  const url = ffInfo.value?.downloadUrl
  if (!url) return
  api.openURL(url).catch((e) => window.alert('打开链接失败：' + (e?.message || e)))
}

function openProgramDir() {
  const dir = ffInfo.value?.programDir
  if (!dir) return
  api.openDirectory(dir).catch((e) => window.alert('打开目录失败：' + (e?.message || e)))
}

async function copyProgramDir() {
  const dir = ffInfo.value?.programDir
  if (!dir) return
  try {
    await navigator.clipboard.writeText(dir)
    copied.value = true
    clearTimeout(copyTimer)
    copyTimer = setTimeout(() => (copied.value = false), 1600)
  } catch {
    // 剪贴板被禁用时至少让路径可见可手抄
    window.alert('复制失败，路径是：\n' + dir)
  }
}

// --- 自动安装 ffmpeg ---

const installing = ref(false)
const installPct = ref(0)
const installMsg = ref('')
const installReceived = ref(0)
const installTotal = ref(0)
const installSpeed = ref(0)
const installError = ref('')
const showManual = ref(false)
const showTried = ref(false)

let offInstall = null

function onInstallEvent(e) {
  // phase: downloading | extracting | done | failed
  if (e.phase === 'done') {
    installing.value = false
    installPct.value = 100
    installMsg.value = ''
    if (e.info) ffInfo.value = e.info
    return
  }
  if (e.phase === 'failed') {
    installing.value = false
    installMsg.value = ''
    installError.value = e.message || '安装失败'
    return
  }

  installPct.value = e.percent || 0
  installMsg.value = e.message || ''
  installReceived.value = e.received || 0
  installTotal.value = e.total || 0
  installSpeed.value = e.speed || 0
}

async function startInstall() {
  installError.value = ''
  installMsg.value = ''
  installPct.value = 0
  installReceived.value = 0
  installTotal.value = 0
  installSpeed.value = 0
  installing.value = true
  try {
    await api.installFFmpeg()
  } catch (e) {
    installing.value = false
    installError.value = e?.message || String(e)
  }
}

async function cancelInstall() {
  try {
    await api.cancelFFmpegInstall()
  } finally {
    installing.value = false
    installMsg.value = ''
  }
}

onMounted(() => {
  refresh()
  offInstall = events.EventsOn('ffmpeg:install', onInstallEvent)
})

onUnmounted(() => {
  if (offInstall) offInstall()
})
</script>

<template>
  <TabsRoot v-model="current" orientation="vertical" class="shell">
    <aside class="sidebar">
      <div class="brand">
        <span class="brand-mark">
          <svg
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="1.8"
            stroke-linecap="round"
            stroke-linejoin="round"
            aria-hidden="true"
          >
            <path d="M5 4.5h14v15H5z" />
            <path d="M10.4 9.4v5.2l4.2-2.6z" />
          </svg>
        </span>
        <span class="brand-text">
          <span class="brand-title">FFmpeg Studio</span>
          <small>本地音视频处理</small>
        </span>
      </div>

      <TabsList class="nav" aria-label="功能导航">
        <TabsTrigger
          v-for="t in tabs"
          :key="t.key"
          :value="t.key"
          class="nav-item"
          :title="t.desc"
        >
          <span class="nav-icon">
            <svg
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="1.6"
              stroke-linecap="round"
              stroke-linejoin="round"
              aria-hidden="true"
            >
              <path :d="t.icon" />
            </svg>
          </span>
          <span class="nav-text">
            <span class="nav-label">{{ t.label }}</span>
            <span class="nav-desc">{{ t.desc }}</span>
          </span>
        </TabsTrigger>
      </TabsList>

      <div class="sidebar-foot">
        <div class="env-badge" :class="{ bad: !envOK && !checking }">
          <span class="env-dot"></span>
          <span class="env-text">
            <template v-if="checking">正在检测 ffmpeg…</template>
            <template v-else-if="envOK">
              <b>ffmpeg {{ ffInfo.version }}</b>
              <span class="env-sub">{{ ffInfo.source }}</span>
            </template>
            <template v-else>
              <b>未找到 ffmpeg</b>
              <span class="env-sub">放到程序同级目录即可</span>
            </template>
          </span>
        </div>
        <button class="ghost tiny" style="width: 100%" @click="redetect">重新检测</button>
      </div>
    </aside>

    <main class="main">
      <!-- 没找到 ffmpeg 时挡在最前面，避免用户白填一堆参数 -->
      <div v-if="!checking && !envOK" class="card">
        <h2>还没有 ffmpeg</h2>

        <!-- 只下了一个 ffmpeg.exe 的情况：单独点破，否则用户会以为程序坏了 -->
        <div v-if="ffInfo?.incomplete" class="banner warn" style="margin-top: 12px">
          找到了 <code>{{ ffInfo.incomplete }}</code>，但它旁边没有 <code>ffprobe.exe</code>。
          本程序要靠 ffprobe 读时长和分辨率，单个 <code>ffmpeg.exe</code> 用不了 ——
          请下载下面这个<b>完整包</b>。
        </div>

        <p class="lead">
          转码、压缩、切片都靠 ffmpeg 干活，得先把它装好。
          已经自动找过<b>程序同级目录</b>、常见安装位置和系统 PATH，都没找到。
        </p>

        <ol class="setup-steps">
          <li>
            <span class="step-no">1</span>
            <div class="step-body">
              <b>让程序自己装好</b>
              <p class="tip-line">
                从官方源下载完整包（约 110&nbsp;MB），自动解压到程序目录的 <code>bin</code> 下并立刻启用。
                网速慢的话要几分钟，随时可以取消。
              </p>

              <!-- 安装中：进度 + 取消 -->
              <div v-if="installing" class="install-box">
                <ProgressRoot :model-value="installPct" class="install-bar">
                  <ProgressIndicator class="install-bar-fill" :style="{ width: installPct + '%' }" />
                </ProgressRoot>
                <div class="install-meta">
                  <span class="install-msg">{{ installMsg || '准备中…' }}</span>
                  <span class="mono">
                    {{ installPct.toFixed(0) }}%<template v-if="installReceived">
                      · {{ formatSize(installReceived) }}</template
                    ><template v-if="installSpeed"> · {{ formatSize(installSpeed) }}/s</template>
                  </span>
                </div>
                <button class="tiny" @click="cancelInstall">取消</button>
              </div>

              <div v-else class="row">
                <button class="primary" @click="startInstall">自动下载并安装</button>
                <button @click="showManual = !showManual">
                  {{ showManual ? '收起手动步骤' : '我要手动装' }}
                </button>
              </div>

              <div v-if="installError" class="banner err" style="margin: 10px 0 0">
                自动安装失败：{{ installError }}
                <template v-if="!showManual">
                  <br />网络受限的话可以改用下面的手动步骤。
                </template>
              </div>
            </div>
          </li>
        </ol>

        <!-- 手动安装三步：折叠收放，默认收起 -->
        <CollapsibleRoot v-model:open="showManual">
          <CollapsibleContent class="u-coll-content manual-steps">
            <div class="step-row">
              <span class="step-no">2</span>
              <div class="step-body">
                <b>自己下载完整版</b>
                <p class="tip-line">
                  约 110&nbsp;MB。必须是带 <code>ffprobe</code> 的完整包 ——
                  光有一个 <code>ffmpeg.exe</code> 是没法用的。
                </p>
                <div class="row">
                  <button @click="openDownload">打开下载页</button>
                </div>
              </div>
            </div>

            <div class="step-row">
              <span class="step-no">3</span>
              <div class="step-body">
                <b>解压后整个文件夹丢进这里</b>
                <p class="tip-line">
                  不用手动挑 exe，程序会自己往下找两三层，所以带目录名也没关系。
                </p>
                <div class="path-row">
                  <code :title="ffInfo?.programDir">{{ ffInfo?.programDir || '（没取到程序目录）' }}</code>
                  <button class="tiny" :disabled="!ffInfo?.programDir" @click="openProgramDir">打开</button>
                  <button class="tiny" :disabled="!ffInfo?.programDir" @click="copyProgramDir">
                    {{ copied ? '已复制' : '复制' }}
                  </button>
                </div>
              </div>
            </div>

            <div class="step-row">
              <span class="step-no">4</span>
              <div class="step-body">
                <b>回到这里重新检测</b>
                <p class="tip-line">放好之后点一下，几秒内就能认出来。</p>
                <div class="row">
                  <button class="primary" :disabled="redetecting" @click="redetect">
                    {{ redetecting ? '检测中…' : '重新检测' }}
                  </button>
                  <button :disabled="setting" @click="browseFFmpeg">
                    {{ setting ? '校验中…' : '我已经有了，手动选择' }}
                  </button>
                </div>
              </div>
            </div>
          </CollapsibleContent>
        </CollapsibleRoot>

        <CollapsibleRoot v-model:open="showTried" class="probe-detail">
          <CollapsibleTrigger class="probe-summary">
            <span class="caret" :class="{ open: showTried }">›</span>
            照做了还是认不出来？
          </CollapsibleTrigger>
          <CollapsibleContent class="u-coll-content">
            <p class="tip-line">这次一共找过这些位置（顺序即优先级）：</p>
            <ul>
              <li v-for="(p, i) in ffInfo?.tried || []" :key="i"><code>{{ p }}</code></li>
            </ul>
          </CollapsibleContent>
        </CollapsibleRoot>
      </div>

      <AnimatePresence v-else mode="wait">
        <motion.div
          :key="current"
          class="page"
          :initial="{ opacity: 0, y: 12 }"
          :animate="{ opacity: 1, y: 0 }"
          :exit="{ opacity: 0, y: -8 }"
          :transition="{ duration: 0.16, ease: 'easeOut' }"
        >
          <div class="page-head">
            <h1>{{ currentTab.label }}</h1>
            <p>{{ currentTab.desc }}</p>
          </div>

          <component :is="currentTab.comp" />
        </motion.div>
      </AnimatePresence>
    </main>
  </TabsRoot>
</template>
