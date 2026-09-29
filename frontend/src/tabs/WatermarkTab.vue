<script setup>
import { ref, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'
import FilePicker from '../components/FilePicker.vue'
import MediaInfoCard from '../components/MediaInfoCard.vue'
import CommandPreview from '../components/CommandPreview.vue'
import ProgressPanel from '../components/ProgressPanel.vue'
import { api, formatTime } from '../api'
import { useTask } from '../composables/useTask'
import { useMediaSource, useCommandPreview, baseFields } from '../composables/useTab'
// 时间段计算抽在 composables/watermarkTime.js，有独立的 node --test 用例
import {
  clamp,
  spanOf,
  inRange,
  applyResize,
  applyMove,
  timeSummary,
  targetSeekFor,
} from '../composables/watermarkTime'
// 水印缩放的几何计算，同样有独立的 node --test 用例
import { scaleFromDrag, IMAGE_RATIO_RANGE, TEXT_RATIO_RANGE } from '../composables/watermarkLayout'

const { file, info, loadingInfo, probeError, outPath, previewURL, previewNote, preparing, load } = useMediaSource('watermark', {
  withPreview: true,
})

const stageEl = ref(null)
const stageBoxEl = ref(null)
const items = ref([])
const selectedId = ref('')
const showSafe = ref(false)
const targetMode = ref('source') // source | 720 | 1080
let seq = 0

// 预览舞台的像素尺寸。
// 必须由 JS 精确算出，不能靠 CSS 撑开：只要舞台尺寸和视频内容区域有偏差，
// 水印位置就会整体偏移，所见即所得就失效了。
const stageSize = ref({ w: 0, h: 0 })

function relayout() {
  const box = stageBoxEl.value
  if (!box) return
  const avail = box.clientWidth || 640
  const vw = info.value?.video?.width || 16
  const vh = info.value?.video?.height || 9
  const maxH = Math.min(window.innerHeight * 0.6, 500)

  let w = avail
  let h = (w * vh) / vw
  if (h > maxH) {
    h = maxH
    w = (h * vw) / vh
  }
  stageSize.value = { w: Math.round(w), h: Math.round(h) }
}

let ro = null
function observeBox() {
  if (ro) ro.disconnect()
  if (!stageBoxEl.value || typeof ResizeObserver === 'undefined') return
  ro = new ResizeObserver(relayout)
  ro.observe(stageBoxEl.value)
}

watch(
  () => [previewURL.value, info.value?.video?.width, info.value?.video?.height],
  async () => {
    await nextTick()
    relayout()
    observeBox()
  }
)

onMounted(() => {
  window.addEventListener('resize', relayout)
})

onUnmounted(() => {
  window.removeEventListener('resize', relayout)
  if (ro) ro.disconnect()
})

const selected = computed(() => items.value.find((i) => i.id === selectedId.value) || null)
const activeCount = computed(() => items.value.filter((i) => i.enabled).length)

// 预览里的画面尺寸：位置和大小全部按比例存放，所以改输出分辨率也不会跑偏
const outW = computed(() => {
  if (targetMode.value === '720') return 1280
  if (targetMode.value === '1080') return 1920
  return info.value?.video?.width || 1920
})
const outH = computed(() => {
  if (targetMode.value === '720') return 720
  if (targetMode.value === '1080') return 1080
  return info.value?.video?.height || 1080
})

/* ---------- 时间段：每个水印各自独立 ---------- */

const videoEl = ref(null)
const tlEl = ref(null)
const currentTime = ref(0)
const duration = computed(() => info.value?.duration || 0)

// 时间轴上的临时提示：写给"拖了但没反应"的情况，
// 否则用户只会以为这个功能坏了（撞到片头/片尾时本来就没有可拖的余地）。
const tlTip = ref('')
let tlTipTimer = null
function showTip(msg) {
  tlTip.value = msg
  clearTimeout(tlTipTimer)
  tlTipTimer = setTimeout(() => {
    tlTip.value = ''
  }, 2800)
}

onUnmounted(() => clearTimeout(tlTipTimer))

function onTimeUpdate() {
  currentTime.value = videoEl.value?.currentTime || 0
}

function isVisible(it) {
  return it.enabled && inRange(it, currentTime.value, duration.value)
}

function summaryOf(it) {
  return timeSummary(it, duration.value)
}

function barStyle(it) {
  const { start, end, duration: d } = spanOf(it, duration.value)
  const s = clamp(start, 0, d)
  const e = clamp(end, 0, d)
  return { left: (s / d) * 100 + '%', width: Math.max(1, ((e - s) / d) * 100) + '%' }
}

function playheadStyle() {
  const d = duration.value || 1
  return { left: clamp(currentTime.value / d, 0, 1) * 100 + '%' }
}

function seekTo(t) {
  const v = videoEl.value
  if (v) v.currentTime = clamp(t, 0, duration.value || t)
  currentTime.value = t
}

// 时间轴上按住拖动 = 拖播放头（点一下也能直接跳过去）。
// 必须挂在 window 上监听移动，否则鼠标一旦滑出色条/红线就断了。
function startScrub(e) {
  e.preventDefault()
  const rect = tlEl.value.getBoundingClientRect()
  const d = duration.value || 0
  if (!d || !rect.width) return

  const apply = (clientX) => {
    const ratio = clamp((clientX - rect.left) / rect.width, 0, 1)
    const t = ratio * d
    // 拖动过程会高频触发，差得不多就别反复 seek，免得视频一直重定位
    if (Math.abs(t - currentTime.value) < 0.02) return
    seekTo(t)
  }

  apply(e.clientX)
  const move = (ev) => apply(ev.clientX)
  const up = () => {
    window.removeEventListener('pointermove', move)
    window.removeEventListener('pointerup', up)
  }
  window.addEventListener('pointermove', move)
  window.addEventListener('pointerup', up)
}

// 调完时间段后，如果画面正停在这个水印看不到的时刻，就自动跳过去，
// 省得用户还要手动拖进度条确认效果。已经在区间里则不动，避免打断播放。
function afterTimeEdit(it) {
  const t = videoEl.value?.currentTime ?? currentTime.value
  const target = targetSeekFor(it, t, duration.value)
  if (target !== null) seekTo(target)
}

// 色条内的文字：显示这段的时间范围，太窄时 CSS 会自然裁掉
function barLabel(it) {
  if (!it.hasTime) return '全程'
  const { start, end } = spanOf(it, duration.value)
  return `${start.toFixed(1)} – ${end.toFixed(1)}`
}

// 拖动色条两端的手柄 → 调整开始 / 消失时间
function resizeBar(it, side, e) {
  e.preventDefault()
  e.stopPropagation()
  selectedId.value = it.id

  const rect = tlEl.value.getBoundingClientRect()
  const d = duration.value || 1
  // 以按下瞬间的区间为基准；否则每次 move 都基于上一帧结果累加，会越拖越快
  const base = spanOf(it, d)
  const wasLimited = !!it.hasTime
  const startX = e.clientX
  let activated = false

  const move = (ev) => {
    const dx = ev.clientX - startX
    // 位移太小一律当点击：不激活，也就不会改动数据（避免"点一下色条就变短"）
    if (!activated) {
      if (Math.abs(dx) < 3) return
      activated = true
      it.hasTime = true
    }

    const delta = (dx / rect.width) * d
    const next = applyResize(base, side, delta, d)
    it.start = next.start
    it.end = next.end
  }

  const up = () => {
    window.removeEventListener('pointermove', move)
    window.removeEventListener('pointerup', up)
    if (!activated) return

    // 拖到头了就不动，但要说明原因 —— 否则看起来就是"拖不动"
    const now = spanOf(it, d)
    if (now.start === base.start && now.end === base.end) {
      // 没产生任何变化就把 hasTime 还原，免得平白留下一个"0–片尾"的限定
      if (!wasLimited) it.hasTime = false
      if (side === 'end') {
        showTip(
          base.end >= d - 0.001
            ? '右端已是片尾，没法再往后拉；往左拖可以提前结束'
            : '区间已经缩到最短了'
        )
      } else {
        showTip(base.start <= 0.001 ? '左端已是片头，没法再往前；往右拖可以推迟出现' : '区间已经缩到最短了')
      }
      return
    }
    afterTimeEdit(it)
  }
  window.addEventListener('pointermove', move)
  window.addEventListener('pointerup', up)
}

// 拖动色条中段 → 整体平移（只影响它自己）。
// 「全程」水印没有可平移的余地，所以给一句提示，免得用户以为拖不动。
function moveBar(it, e) {
  e.preventDefault()
  e.stopPropagation()
  selectedId.value = it.id

  if (!it.hasTime) {
    showTip('这是「全程」水印：拖色条两端的手柄，可以限定它出现的时间段')
    return
  }

  const rect = tlEl.value.getBoundingClientRect()
  const d = duration.value || 1
  const base = spanOf(it, d)
  const startX = e.clientX
  let activated = false

  const move = (ev) => {
    const dx = ev.clientX - startX
    if (!activated) {
      if (Math.abs(dx) < 3) return
      activated = true
    }
    const delta = (dx / rect.width) * d
    const next = applyMove(base, delta, d)
    it.start = next.start
    it.end = next.end
  }

  const up = () => {
    window.removeEventListener('pointermove', move)
    window.removeEventListener('pointerup', up)
    if (!activated) return

    const now = spanOf(it, d)
    if (now.start === base.start && now.end === base.end) {
      showTip('这一段已经贴到片头或片尾了，没法再往这个方向平移')
      return
    }
    afterTimeEdit(it)
  }
  window.addEventListener('pointermove', move)
  window.addEventListener('pointerup', up)
}

function clearTime(it) {
  it.hasTime = false
  it.start = 0
  it.end = 0
}

function addImage(wm) {
  items.value.push({
    id: `w${++seq}`,
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
  })
  selectedId.value = items.value[items.value.length - 1].id
}

function addText() {
  items.value.push({
    id: `w${++seq}`,
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
  })
  selectedId.value = items.value[items.value.length - 1].id
}

async function pickImage() {
  const f = await api.pickImageFile()
  if (f) addImage(f)
}

async function pickFont() {
  const f = await api.pickFontFile()
  if (f && selected.value) selected.value.fontFile = f.path
}

function remove(id) {
  items.value = items.value.filter((i) => i.id !== id)
  if (selectedId.value === id) selectedId.value = items.value[0]?.id || ''
}

function duplicate(id) {
  const src = items.value.find((i) => i.id === id)
  if (!src) return
  const copy = { ...src, id: `w${++seq}`, x: clamp(src.x + 0.03, 0, 0.95), y: clamp(src.y + 0.03, 0, 0.95) }
  items.value.splice(items.value.indexOf(src) + 1, 0, copy)
  selectedId.value = copy.id
}

// 顺序即层叠顺序：靠后的盖在靠前的上面
function move(id, dir) {
  const i = items.value.findIndex((x) => x.id === id)
  const j = i + dir
  if (i < 0 || j < 0 || j >= items.value.length) return
  const arr = items.value
  ;[arr[i], arr[j]] = [arr[j], arr[i]]
}

/* ---------- 拖拽定位 ---------- */
function startDrag(item, e) {
  e.preventDefault()
  e.stopPropagation()
  selectedId.value = item.id
  const rect = stageEl.value.getBoundingClientRect()
  const sx = e.clientX
  const sy = e.clientY
  const ox = item.x
  const oy = item.y

  const move = (ev) => {
    item.x = clamp(ox + (ev.clientX - sx) / rect.width, 0, 1)
    item.y = clamp(oy + (ev.clientY - sy) / rect.height, 0, 1)
  }
  const up = () => {
    window.removeEventListener('pointermove', move)
    window.removeEventListener('pointerup', up)
  }
  window.addEventListener('pointermove', move)
  window.addEventListener('pointerup', up)
}

function startResize(item, e) {
  e.preventDefault()
  e.stopPropagation()
  selectedId.value = item.id

  // 量出水印元素当前的实际渲染宽度，作为整个拖动过程的缩放基准。
  // 文字水印的渲染宽度与字号并不成正比（三个字、字号 36px 时实际宽约 108px），
  // 直接拿字号当宽度算，手柄会跑得比鼠标快好几倍。
  const el = e.currentTarget?.parentElement
  const startWidth = el?.getBoundingClientRect().width || 0
  const baseRatio = item.kind === 'image' ? item.wRatio : item.fontSizeRatio
  const sx = e.clientX

  const move = (ev) => {
    const dx = ev.clientX - sx
    if (item.kind === 'image') {
      item.wRatio = scaleFromDrag(baseRatio, startWidth, dx, ...IMAGE_RATIO_RANGE)
    } else {
      item.fontSizeRatio = scaleFromDrag(baseRatio, startWidth, dx, ...TEXT_RATIO_RANGE)
    }
  }

  const up = () => {
    window.removeEventListener('pointermove', move)
    window.removeEventListener('pointerup', up)
  }
  window.addEventListener('pointermove', move)
  window.addEventListener('pointerup', up)
}

function stageStyle(item) {
  const base = {
    left: `${item.x * 100}%`,
    top: `${item.y * 100}%`,
    cursor: 'move',
  }
  if (item.kind === 'image') {
    base.width = `${item.wRatio * 100}%`
    base.opacity = item.opacity
  } else {
    // 字号用像素写死，等价于「字号 ÷ 画面高」，和导出时的换算完全一致
    base.fontSize = `${Math.max(8, item.fontSizeRatio * stageSize.value.h)}px`
    base.color = item.color
    base.opacity = item.opacity
    base.lineHeight = '1.2'
    if (item.borderW > 0) {
      base.webkitTextStroke = `${item.borderW * 0.5}px ${item.borderColor}`
      base.textShadow = `0 0 ${item.borderW}px ${item.borderColor}`
    }
    if (item.box) {
      base.background = 'rgba(0,0,0,0.5)'
      base.padding = '0.15em 0.4em'
      base.borderRadius = '4px'
    }
  }
  return base
}

/* ---------- 生成任务 ---------- */
function buildSpec() {
  if (!file.value || !outPath.value || !info.value) return null
  if (!activeCount.value) return null

  return {
    kind: 'watermark',
    output: outPath.value,
    ...baseFields(file, info),
    watermark: {
      targetW: targetMode.value === 'source' ? 0 : outW.value,
      targetH: targetMode.value === 'source' ? 0 : outH.value,
      // 只发后端认识的字段，url 是前端预览用的
      items: items.value.map((i) => ({
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
  }
}

const { cmd } = useCommandPreview(buildSpec, [file, outPath, items, targetMode])
const { running, progress, stage, logs, result, start, cancel } = useTask()

async function run() {
  const spec = buildSpec()
  if (!spec) return
  try {
    await start(spec)
  } catch (e) {
    window.alert('无法启动任务：' + (e?.message || e))
  }
}

async function pickOutput() {
  const ext = file.value?.name?.match(/\.[^.]+$/)?.[0] || '.mp4'
  const name = (file.value?.name || 'output').replace(/\.[^.]+$/, '') + '_watermarked' + ext
  const p = await api.pickSaveFile(name, `*${ext}`)
  if (p) outPath.value = p
}

</script>

<template>
  <div class="card">
    <h2>源文件</h2>
    <FilePicker :model-value="file" @update:model-value="load" label="选择视频，也可以直接拖进来" />
    <div v-if="file" style="margin-top: 12px">
      <MediaInfoCard :info="info" :loading="loadingInfo" />
      <div v-if="preparing" class="banner info" style="margin-top: 10px">正在准备预览…</div>
      <div v-else-if="previewNote" class="banner info" style="margin-top: 10px">{{ previewNote }}</div>
      <div v-if="probeError" class="banner warn" style="margin-top: 10px">{{ probeError }}</div>
    </div>
  </div>

  <div v-if="file && previewURL" class="wm-layout">
    <!-- 预览区：水印是叠在视频上的网页元素，拖动零延迟；导出时按同一套比例换算成 ffmpeg 参数 -->
    <div class="card stage-card">
      <h2>
        实时预览
        <span class="hint">拖水印移动位置，拖右下角圆点改大小</span>
      </h2>
      <div ref="stageBoxEl" class="stage-box">
        <div ref="stageEl" class="stage" :style="{ width: stageSize.w + 'px', height: stageSize.h + 'px' }">
          <video
            ref="videoEl"
            :src="previewURL"
            controls
            preload="metadata"
            @timeupdate="onTimeUpdate"
            @seeked="onTimeUpdate"
          />
          <div
            v-for="it in items"
            v-show="isVisible(it)"
            :key="it.id"
            class="wm"
            :class="{ sel: it.id === selectedId, text: it.kind === 'text' }"
            :style="stageStyle(it)"
            @pointerdown="startDrag(it, $event)"
          >
            <img v-if="it.kind === 'image'" :src="it.url" alt="水印" draggable="false" />
            <span v-else>{{ it.text || '文字' }}</span>
            <i class="resize" @pointerdown="startResize(it, $event)"></i>
          </div>
        </div>
      </div>
      <!-- 时间轴：每个水印一条色条，各自的时间段一眼可见、可直接拖动 -->
      <div class="tl">
        <div class="tl-head">
          <span class="hint" :class="{ warn: tlTip }">{{
            tlTip || '时间轴 · 拖红点定位播放位置；拖色条两端调起止时间，拖中段平移'
          }}</span>
          <span class="mono tl-clock">{{ formatTime(currentTime) }} / {{ formatTime(duration) }}</span>
        </div>

        <div ref="tlEl" class="tl-track" @pointerdown="startScrub">
          <div v-if="!items.length" class="tl-empty">还没有水印</div>
          <div
            v-for="it in items"
            :key="it.id"
            class="tl-bar"
            :class="{
              off: !it.enabled,
              sel: it.id === selectedId,
              text: it.kind === 'text',
              full: !it.hasTime,
            }"
            :style="barStyle(it)"
            :title="`${it.kind === 'image' ? '图片水印' : it.text || '文字水印'} · ${summaryOf(it)}`"
            @pointerdown="moveBar(it, $event)"
          >
            <span class="tl-grip left" title="拖动这里：调整开始时间" @pointerdown="resizeBar(it, 'start', $event)"></span>
            <span class="tl-label">{{ barLabel(it) }}</span>
            <span class="tl-grip right" title="拖动这里：调整消失时间" @pointerdown="resizeBar(it, 'end', $event)"></span>
          </div>
          <div class="tl-playhead" :style="playheadStyle()">
            <span
              class="tl-knob"
              title="按住拖动：移动播放位置"
              @pointerdown.stop="startScrub($event)"
            ></span>
          </div>
        </div>

        <div class="tl-scale mono">
          <span>0:00</span>
          <span>{{ formatTime(duration / 2) }}</span>
          <span>{{ formatTime(duration) }}</span>
        </div>
      </div>

      <div class="stage-foot">
        <span class="hint">预览仅用于定位，最终效果以导出为准</span>
      </div>
    </div>

    <!-- 右侧：水印列表 + 选中项属性 -->
    <div class="side">
      <div class="card">
        <h2>
          水印列表
          <span class="hint">越靠后越在上层</span>
        </h2>

        <div v-if="!items.length" class="empty-hint" style="padding: 18px">还没有水印</div>

        <div v-else class="list">
          <div
            v-for="(it, idx) in items"
            :key="it.id"
            class="listitem"
            :class="{ on: it.id === selectedId }"
            @click="selectedId = it.id"
          >
            <input v-model="it.enabled" type="checkbox" @click.stop />
            <span class="li-body">
              <span class="li-name">{{ it.kind === 'image' ? '图片水印' : it.text || '文字水印' }}</span>
              <span class="li-time" :class="{ limited: it.hasTime }">{{ summaryOf(it) }}</span>
            </span>
            <span class="li-ops">
              <button class="ghost tiny" :disabled="idx === 0" @click.stop="move(it.id, -1)" title="上移">↑</button>
              <button class="ghost tiny" :disabled="idx === items.length - 1" @click.stop="move(it.id, 1)" title="下移">↓</button>
              <button class="ghost tiny" @click.stop="duplicate(it.id)" title="复制">⧉</button>
              <button class="ghost tiny" @click.stop="remove(it.id)" title="删除">×</button>
            </span>
          </div>
        </div>

        <div class="row" style="margin-top: 10px">
          <button class="tiny" @click="pickImage">+ 图片水印</button>
          <button class="tiny" @click="addText">+ 文字水印</button>
        </div>
      </div>

      <div v-if="selected" class="card">
        <h2>选中项设置</h2>

        <template v-if="selected.kind === 'image'">
          <div class="field">
            <label>图片</label>
            <div class="row">
              <input :value="selected.path" type="text" class="mono" style="flex: 1" readonly />
              <button class="tiny" @click="pickImage">换一张</button>
            </div>
          </div>
        </template>

        <template v-else>
          <div class="field">
            <label>文字内容</label>
            <input v-model="selected.text" type="text" placeholder="要显示的文字" />
          </div>
          <div class="field" style="margin-top: 10px">
            <label>字体文件 <span class="tip">留空则自动选系统字体</span></label>
            <div class="row">
              <input :value="selected.fontFile || '（自动）'" type="text" class="mono" style="flex: 1" readonly />
              <button class="tiny" @click="pickFont">选择…</button>
            </div>
          </div>
          <div class="grid2" style="margin-top: 10px">
            <div class="field">
              <label>文字颜色</label>
              <input v-model="selected.color" type="color" class="color" />
            </div>
            <div class="field">
              <label>描边宽度</label>
              <input v-model.number="selected.borderW" type="number" min="0" max="10" />
            </div>
          </div>
          <div class="grid2" style="margin-top: 10px">
            <div class="field">
              <label>描边颜色</label>
              <input v-model="selected.borderColor" type="color" class="color" />
            </div>
            <div class="field" style="justify-content: flex-end">
              <label class="check">
                <input v-model="selected.box" type="checkbox" />
                加背景框
              </label>
            </div>
          </div>
        </template>

        <div class="divider"></div>

        <div class="field">
          <label>不透明度 {{ Math.round(selected.opacity * 100) }}%</label>
          <input v-model.number="selected.opacity" type="range" min="0.05" max="1" step="0.01" />
        </div>

        <template v-if="selected.hasTime">
          <div class="divider"></div>
          <div class="grid2">
            <div class="field">
              <label>起始（秒）</label>
              <input v-model.number="selected.start" type="number" min="0" step="0.1" @change="afterTimeEdit(selected)" />
            </div>
            <div class="field">
              <label>结束（秒，0 = 到片尾）</label>
              <input v-model.number="selected.end" type="number" min="0" step="0.1" @change="afterTimeEdit(selected)" />
            </div>
          </div>
          <div class="row" style="margin-top: 8px">
            <button class="tiny" @click="clearTime(selected)">取消时间限制</button>
          </div>
        </template>
      </div>

      <div class="card">
        <h2>输出设置</h2>
        <div class="grid2">
          <div class="field">
            <label>输出分辨率</label>
            <select v-model="targetMode">
              <option value="source">与源相同</option>
              <option value="720">720p（1280×720）</option>
              <option value="1080">1080p（1920×1080）</option>
            </select>
          </div>
          <div class="field">
            <label>水印数量</label>
            <div class="calc mono">{{ activeCount }} 个生效</div>
          </div>
        </div>

        <div v-if="activeCount > 5" class="banner warn" style="margin-top: 12px; margin-bottom: 0">
          水印较多，每个水印都要叠一次，编码速度会明显下降。
        </div>
      </div>
    </div>
  </div>

  <div v-if="file" class="card">
    <h2>输出</h2>
    <div class="field">
      <label>输出路径</label>
      <div class="row">
        <input v-model="outPath" type="text" class="mono" style="flex: 1" />
        <button @click="pickOutput">浏览…</button>
      </div>
    </div>

    <CommandPreview :steps="cmd.steps" :err="cmd.err" />

    <div class="row" style="margin-top: 14px">
      <button class="primary" :disabled="running || !outPath || !activeCount" @click="run">
        {{ running ? '处理中…' : `开始导出（${activeCount} 个水印）` }}
      </button>
    </div>
  </div>

  <ProgressPanel
    :running="running"
    :progress="progress"
    :stage="stage"
    :logs="logs"
    :result="result"
    @cancel="cancel"
    @open-file="api.openPath"
    @open-dir="api.openDirectory"
  />
</template>

<style scoped>
.wm-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 330px;
  gap: 14px;
  align-items: start;
}

.stage-card {
  margin-bottom: 0;
}

.stage-box {
  display: flex;
  justify-content: center;
  background: #f0f2f5;
  border-radius: var(--radius-sm);
  padding: 8px;
  min-height: 120px;
}

.stage {
  position: relative;
  background: #000;
  border-radius: var(--radius-sm);
  overflow: hidden;
  line-height: 0;
  flex: none;
}

.stage video {
  width: 100%;
  height: 100%;
  display: block;
  object-fit: fill;
}

.wm {
  position: absolute;
  user-select: none;
  touch-action: none;
  outline: 1px dashed transparent;
  transition: outline-color 0.12s;
}

.wm.sel {
  outline-color: #38bdf8;
}

.wm img {
  width: 100%;
  display: block;
  pointer-events: none;
}

.wm.text span {
  display: inline-block;
  white-space: pre;
  font-weight: 500;
}

.resize {
  position: absolute;
  right: -6px;
  bottom: -6px;
  width: 11px;
  height: 11px;
  border-radius: 50%;
  background: #38bdf8;
  border: 1.5px solid #fff;
  cursor: nwse-resize;
  display: none;
}

.wm.sel .resize {
  display: block;
}

.stage-foot {
  margin-top: 8px;
}

/* ---------- 时间轴 ---------- */

.tl {
  margin-top: 12px;
}

.tl-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 6px;
}

.tl-clock {
  font-size: 11.5px;
  color: var(--text-dim);
  flex: none;
}

/* "拖不动"或"这是全程水印"这类临时提示，用暖色标出来 */
.hint.warn {
  color: #b45309;
  font-weight: 500;
}

.tl-track {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 7px 0;
  background: #f2f4f7;
  border: 1px solid var(--border);
  border-radius: 6px;
  cursor: crosshair;
  touch-action: none;
  min-height: 40px;
}

.tl-empty {
  padding: 3px 10px;
  font-size: 11.5px;
  color: var(--text-mute);
}

.tl-bar {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 20px;
  border-radius: 4px;
  background: #a5b4fc;
  color: #312e81;
  font-size: 10.5px;
  line-height: 1;
  cursor: grab;
  overflow: hidden;
  position: relative;
  z-index: 1;
  user-select: none;
  /* 两端手柄各 13px，太窄就抓不到了 */
  min-width: 34px;
}

.tl-bar.text {
  background: #fcd34d;
  color: #78350f;
}

/* 还没限定时间段的（全程）：斜纹底 + 明确文字，
   和已设过区间的实心色条一眼区分开，暗示"这条还能拖" */
.tl-bar.full {
  background: repeating-linear-gradient(45deg, #c7d2fe, #c7d2fe 6px, #e0e7ff 6px, #e0e7ff 12px);
  color: #3730a3;
}

.tl-bar.full.text {
  background: repeating-linear-gradient(45deg, #fcd34d, #fcd34d 6px, #fdeaaa 6px, #fdeaaa 12px);
  color: #78350f;
}

.tl-bar.sel {
  outline: 2px solid var(--primary);
  outline-offset: -1px;
}

.tl-bar.off {
  opacity: 0.3;
}

.tl-bar:active {
  cursor: grabbing;
}

/* 两端拖拽手柄：左端 = 开始时间，右端 = 消失时间。
   做宽一点、加个竖向握把，否则 8px 宽根本不好抓。 */
.tl-grip {
  position: absolute;
  top: 0;
  bottom: 0;
  width: 13px;
  background: rgba(0, 0, 0, 0.26);
  cursor: ew-resize;
  z-index: 3;
  transition: background 0.12s;
}

.tl-grip::after {
  content: '';
  position: absolute;
  left: 50%;
  top: 50%;
  width: 2px;
  height: 10px;
  transform: translate(-50%, -50%);
  border-radius: 1px;
  background: rgba(255, 255, 255, 0.9);
}

.tl-grip.left {
  left: 0;
  border-radius: 4px 0 0 4px;
}

.tl-grip.right {
  right: 0;
  border-radius: 0 4px 4px 0;
}

.tl-grip:hover {
  background: var(--primary);
}

.tl-label {
  padding: 0 16px;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
  pointer-events: none;
}

.tl-playhead {
  position: absolute;
  top: 0;
  bottom: 0;
  width: 2px;
  margin-left: -1px;
  background: #ef4444;
  z-index: 2;
  pointer-events: none;
}

/* 红线顶端的圆形把手。
   红线本体刻意不接指针（pointer-events:none），免得挡住下面色条的拖拽，
   所以单独做一个小圆点专门用来抓。 */
.tl-knob {
  position: absolute;
  left: 50%;
  top: -4px;
  width: 12px;
  height: 12px;
  margin-left: -6px;
  border-radius: 50%;
  background: #ef4444;
  border: 2px solid #fff;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.35);
  pointer-events: auto;
  cursor: ew-resize;
  z-index: 4;
  transition: transform 0.1s;
}

.tl-knob:hover {
  transform: scale(1.15);
}

.tl-scale {
  display: flex;
  justify-content: space-between;
  margin-top: 4px;
  font-size: 10.5px;
  color: var(--text-mute);
}

.side {
  display: flex;
  flex-direction: column;
}

.list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: 210px;
  overflow-y: auto;
}

.listitem {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 8px;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  cursor: pointer;
  font-size: 12px;
}

.listitem.on {
  border-color: var(--primary);
  background: var(--primary-soft);
}

.li-body {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  line-height: 1.35;
}

.li-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.li-time {
  font-size: 10.5px;
  color: var(--text-mute);
}

/* 有时间限制的水印用暖色标出来，和"全程"一眼区分 */
.li-time.limited {
  color: #b45309;
  font-weight: 500;
}

.li-ops {
  display: flex;
  gap: 1px;
  opacity: 0.55;
}

.listitem:hover .li-ops {
  opacity: 1;
}

.divider {
  height: 1px;
  background: var(--border);
  margin: 13px 0;
}

.color {
  padding: 2px;
  height: 32px;
  cursor: pointer;
}

.calc {
  padding: 7px 10px;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: #f7f8fa;
  font-size: 13px;
  font-weight: 500;
}

.mono {
  font-size: 12px;
}
</style>
