<script setup>
import { ref, computed } from 'vue'
import { SliderRoot, SliderTrack, SliderRange, SliderThumb } from 'reka-ui'
import AppNumber from '../ui/AppNumber.vue'
import { formatTime } from '../api'

// 可视化时间轴：截取片段、抽帧、GIF 三个 Tab 共用。
// mode="range" 选区间（双滑块），mode="point" 选单点（单滑块）。
// 拖动逻辑交给 Reka Slider（含键盘方向键、点击定位），这里只负责把值同步给视频预览
const props = defineProps({
  src: { type: String, required: true },
  duration: { type: Number, default: 0 },
  mode: { type: String, default: 'range' },
  modelValue: { type: Object, default: () => ({ start: 0, end: 0, time: 0 }) },
})
const emit = defineEmits(['update:modelValue'])

const videoEl = ref(null)

const startVal = computed(() => (props.mode === 'range' ? Number(props.modelValue.start ?? 0) : 0))
const endVal = computed(() =>
  props.mode === 'range' ? Number(props.modelValue.end ?? props.duration) : Number(props.modelValue.time ?? 0)
)

// Reka 要求 max > min：元数据还没读到时先给一个可用的刻度
const maxVal = computed(() => Math.max(props.duration || 0, 0.1))

const sliderValue = computed(() =>
  props.mode === 'range' ? [startVal.value, endVal.value] : [endVal.value]
)

function seekTo(t) {
  const v = videoEl.value
  if (v) v.currentTime = Math.max(0, Math.min(t, props.duration || t))
}

// 滑块拖动：找出是哪个手柄动了，视频实时跟到那个位置
function onUpdate(v) {
  const arr = Array.isArray(v) ? v : [v]
  if (!arr.length || arr.some((x) => typeof x !== 'number')) return

  if (props.mode === 'range') {
    const movedStart = Math.abs(arr[0] - startVal.value) >= Math.abs(arr[1] - endVal.value)
    const next = { ...props.modelValue, start: arr[0], end: arr[1] }
    emit('update:modelValue', next)
    seekTo(movedStart ? arr[0] : arr[1])
  } else {
    emit('update:modelValue', { ...props.modelValue, time: arr[0] })
    seekTo(arr[0])
  }
}

function onMeta() {
  // 时长以探测结果为准，但视频元数据更准时同步一次
  const v = videoEl.value
  if (!v) return
  if (props.mode === 'range' && (!props.modelValue.end || props.modelValue.end > v.duration)) {
    emit('update:modelValue', { ...props.modelValue, end: v.duration })
  }
}

function setStart(val) {
  const n = Number(val) || 0
  emit('update:modelValue', { ...props.modelValue, start: clamp(n) })
  seekTo(n)
}
function setEnd(val) {
  const n = Number(val) || 0
  emit('update:modelValue', { ...props.modelValue, end: clamp(n) })
  seekTo(n)
}
function setPoint(val) {
  const n = Number(val) || 0
  emit('update:modelValue', { ...props.modelValue, time: clamp(n) })
  seekTo(n)
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
      <SliderRoot
        class="track"
        orientation="horizontal"
        :model-value="sliderValue"
        :min="0"
        :max="maxVal"
        :step="0.01"
        :min-steps-between-thumbs="5"
        @update:model-value="onUpdate"
      >
        <SliderTrack class="rail">
          <SliderRange class="sel" />
        </SliderTrack>
        <SliderThumb class="handle" aria-label="时间手柄" />
        <SliderThumb v-if="mode === 'range'" class="handle" aria-label="结束时间" />
      </SliderRoot>
    </div>

    <div class="times">
      <template v-if="mode === 'range'">
        <div class="tfield">
          <label>起点（秒）</label>
          <AppNumber :model-value="startVal" :min="0" :max="maxVal" :step="0.01" @update:model-value="setStart" />
        </div>
        <div class="tfield">
          <label>终点（秒）</label>
          <AppNumber :model-value="endVal" :min="0" :max="maxVal" :step="0.01" @update:model-value="setEnd" />
        </div>
        <div class="readout">
          选中 <b>{{ formatTime(Math.max(0, endVal - startVal)) }}</b>
          <span class="dim">/ 全片 {{ formatTime(duration) }}</span>
        </div>
      </template>

      <template v-else>
        <div class="tfield">
          <label>时间点（秒）</label>
          <AppNumber :model-value="endVal" :min="0" :max="maxVal" :step="0.01" @update:model-value="setPoint" />
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

/* Reka 的 SliderRoot 默认渲染成 <span>，手柄是内联的 position:absolute + left:%。
   这里必须自己给它定位上下文（否则手柄会以整个窗口为基准，飘到左下角），
   同时用 flex 把 span 变成块级容器，rail 才有高度/宽度。与全局 .u-slider 保持一致 */
.track {
  position: relative;
  display: flex;
  align-items: center;
  height: 22px;
  cursor: pointer;
  touch-action: none;
  user-select: none;
}

.rail {
  position: relative;
  width: 100%;
  height: 6px;
  border-radius: 3px;
  background: #e6e9ee;
}

.sel {
  position: absolute;
  height: 100%;
  border-radius: 3px;
  background: #93b4f7;
}

.handle {
  display: block;
  width: 14px;
  height: 14px;
  border-radius: 50%;
  background: #fff;
  border: 2px solid var(--primary);
  box-shadow: 0 1px 3px rgba(15, 23, 42, 0.2);
  cursor: grab;
  outline: none;
  transition: transform 0.1s, box-shadow 0.1s;
}

.handle:hover {
  transform: var(--reka-slider-thumb-transform) scale(1.12);
}

.handle:active {
  cursor: grabbing;
}

.handle:focus-visible {
  box-shadow: 0 0 0 4px rgba(37, 99, 235, 0.2);
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
  width: 130px;
}

.tfield label {
  font-size: 11px;
  color: var(--text-mute);
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
