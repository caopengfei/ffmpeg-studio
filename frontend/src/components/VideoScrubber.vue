<script setup>
import { ref, computed } from 'vue'
import { formatTime } from '../api'

// 可视化时间轴：截取片段、抽帧、GIF 三个 Tab 共用。
// mode="range" 选区间，mode="point" 选单点。
const props = defineProps({
  src: { type: String, required: true },
  duration: { type: Number, default: 0 },
  mode: { type: String, default: 'range' },
  modelValue: { type: Object, default: () => ({ start: 0, end: 0, time: 0 }) },
})
const emit = defineEmits(['update:modelValue'])

const videoEl = ref(null)
const trackEl = ref(null)
const dragging = ref('')

const startVal = computed(() => (props.mode === 'range' ? Number(props.modelValue.start ?? 0) : 0))
const endVal = computed(() =>
  props.mode === 'range' ? Number(props.modelValue.end ?? props.duration) : Number(props.modelValue.time ?? 0)
)

const startPct = computed(() => pct(startVal.value))
const endPct = computed(() => pct(endVal.value))

function pct(t) {
  if (!props.duration) return 0
  return Math.max(0, Math.min(100, (t / props.duration) * 100))
}

function seekTo(t) {
  const v = videoEl.value
  if (v) v.currentTime = Math.max(0, Math.min(t, props.duration || t))
}

function timeAt(clientX) {
  const rect = trackEl.value.getBoundingClientRect()
  const ratio = Math.max(0, Math.min(1, (clientX - rect.left) / rect.width))
  return ratio * (props.duration || 0)
}

function onDown(which, e) {
  e.preventDefault()
  dragging.value = which
  trackEl.value.setPointerCapture(e.pointerId)
  apply(e)
}

function apply(e) {
  const t = timeAt(e.clientX)
  const next = { ...props.modelValue }

  if (dragging.value === 'start') {
    next.start = Math.min(t, endVal.value - 0.05)
    seekTo(next.start)
  } else if (dragging.value === 'end') {
    next.end = Math.max(t, startVal.value + 0.05)
    seekTo(next.end)
  } else {
    next.time = t
    seekTo(t)
  }
  emit('update:modelValue', next)
}

function onMove(e) {
  if (dragging.value) apply(e)
}

function onUp(e) {
  if (dragging.value) {
    try {
      trackEl.value.releasePointerCapture(e.pointerId)
    } catch {
      /* 指针已释放，忽略 */
    }
  }
  dragging.value = ''
}

function onTrackDown(e) {
  if (dragging.value) return
  const t = timeAt(e.clientX)
  seekTo(t)
}

function onMeta() {
  // 时长以探测结果为准，但视频元数据更准时同步一次
  const v = videoEl.value
  if (!v) return
  if (props.mode === 'range' && (!props.modelValue.end || props.modelValue.end > v.duration)) {
    emit('update:modelValue', { ...props.modelValue, end: v.duration })
  }
}

function setStart(v) {
  emit('update:modelValue', { ...props.modelValue, start: clamp(Number(v)) })
  seekTo(Number(v))
}
function setEnd(v) {
  emit('update:modelValue', { ...props.modelValue, end: clamp(Number(v)) })
  seekTo(Number(v))
}
function setPoint(v) {
  emit('update:modelValue', { ...props.modelValue, time: clamp(Number(v)) })
  seekTo(Number(v))
}
function clamp(v) {
  if (!isFinite(v) || v < 0) return 0
  if (props.duration && v > props.duration) return props.duration
  return v
}
</script>

<template>
  <div class="scrubber">
    <video ref="videoEl" class="preview" :src="src" controls preload="metadata" @loadedmetadata="onMeta" />

    <div class="track-row">
      <div
        ref="trackEl"
        class="track"
        @pointerdown="onTrackDown"
        @pointermove="onMove"
        @pointerup="onUp"
        @pointercancel="onUp"
      >
        <div class="rail"></div>

        <template v-if="mode === 'range'">
          <div class="sel" :style="{ left: startPct + '%', width: Math.max(0, endPct - startPct) + '%' }"></div>
          <div class="handle" :style="{ left: startPct + '%' }" @pointerdown="onDown('start', $event)"></div>
          <div class="handle" :style="{ left: endPct + '%' }" @pointerdown="onDown('end', $event)"></div>
        </template>

        <template v-else>
          <div class="handle solo" :style="{ left: endPct + '%' }" @pointerdown="onDown('point', $event)"></div>
        </template>
      </div>
    </div>

    <div class="times">
      <template v-if="mode === 'range'">
        <div class="tfield">
          <label>起点</label>
          <input type="number" step="0.01" min="0" :value="startVal.toFixed(2)" @change="setStart($event.target.value)" />
        </div>
        <div class="tfield">
          <label>终点</label>
          <input type="number" step="0.01" min="0" :value="endVal.toFixed(2)" @change="setEnd($event.target.value)" />
        </div>
        <div class="readout">
          选中 <b>{{ formatTime(Math.max(0, endVal - startVal)) }}</b>
          <span class="dim">/ 全片 {{ formatTime(duration) }}</span>
        </div>
      </template>

      <template v-else>
        <div class="tfield">
          <label>时间点</label>
          <input type="number" step="0.01" min="0" :value="endVal.toFixed(2)" @change="setPoint($event.target.value)" />
        </div>
        <div class="readout">
          <b>{{ formatTime(endVal) }}</b>
          <span class="dim">/ 全片 {{ formatTime(duration) }}</span>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.scrubber {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.preview {
  width: 100%;
  max-height: 340px;
  background: #000;
  border-radius: var(--radius-sm);
  display: block;
}

.track-row {
  padding: 12px 6px;
}

.track {
  position: relative;
  height: 22px;
  cursor: pointer;
  touch-action: none;
}

.rail {
  position: absolute;
  top: 50%;
  transform: translateY(-50%);
  left: 0;
  right: 0;
  height: 6px;
  border-radius: 3px;
  background: #e6e9ee;
}

.sel {
  position: absolute;
  top: 50%;
  transform: translateY(-50%);
  height: 6px;
  border-radius: 3px;
  background: #93b4f7;
}

.handle {
  position: absolute;
  top: 50%;
  width: 14px;
  height: 14px;
  margin-left: -7px;
  transform: translateY(-50%);
  border-radius: 50%;
  background: #fff;
  border: 2px solid var(--primary);
  cursor: grab;
  transition: transform 0.1s;
}

.handle:active {
  cursor: grabbing;
  transform: translateY(-50%) scale(1.15);
}

.handle.solo {
  border-color: #b45309;
}

.times {
  display: flex;
  align-items: flex-end;
  gap: 14px;
  flex-wrap: wrap;
}

.tfield {
  display: flex;
  flex-direction: column;
  gap: 3px;
  width: 110px;
}

.tfield label {
  font-size: 11px;
  color: var(--text-mute);
}

.tfield input {
  padding: 5px 8px;
  font-size: 12px;
}

.readout {
  margin-left: auto;
  font-size: 12px;
  color: var(--text-dim);
}

.readout b {
  color: var(--text);
}

.readout .dim {
  color: var(--text-mute);
}
</style>
