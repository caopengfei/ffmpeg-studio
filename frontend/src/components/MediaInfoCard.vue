<script setup>
import { computed } from 'vue'
import { formatSize, formatDuration } from '../api'

const props = defineProps({
  info: { type: Object, default: null },
  loading: { type: Boolean, default: false },
})

const videoLine = computed(() => {
  const v = props.info?.video
  if (!v) return '—'
  const parts = [`${v.width}×${v.height}`]
  parts.push(v.codec.toUpperCase())
  if (v.fps > 0) parts.push(`${v.fps.toFixed(2)} fps`)
  if (v.bitRate > 0) parts.push(`${Math.round(v.bitRate / 1000)} kbps`)
  return parts.join(' · ')
})

const audioLine = computed(() => {
  const a = props.info?.audio
  if (!a) return '无音频'
  const parts = [a.codec.toUpperCase()]
  if (a.sampleRate) parts.push(`${a.sampleRate} Hz`)
  if (a.channels) parts.push(a.channels === 1 ? '单声道' : a.channels === 2 ? '立体声' : `${a.channels} 声道`)
  if (a.bitRate > 0) parts.push(`${Math.round(a.bitRate / 1000)} kbps`)
  return parts.join(' · ')
})
</script>

<template>
  <div v-if="loading" class="info-loading">正在读取媒体信息…</div>

  <div v-else-if="info" class="info">
    <div class="info-grid">
      <div class="cell">
        <span class="k">时长</span>
        <span class="v">{{ formatDuration(info.duration) }}</span>
      </div>
      <div class="cell">
        <span class="k">大小</span>
        <span class="v">{{ formatSize(info.size) }}</span>
      </div>
      <div class="cell">
        <span class="k">总码率</span>
        <span class="v">{{ info.bitRate > 0 ? Math.round(info.bitRate / 1000) + ' kbps' : '—' }}</span>
      </div>
      <div class="cell">
        <span class="k">容器</span>
        <span class="v">{{ info.format || '—' }}</span>
      </div>
    </div>

    <div class="streams">
      <div class="stream">
        <span class="tag">视频</span>
        <span class="mono">{{ videoLine }}</span>
      </div>
      <div class="stream">
        <span class="tag">音频</span>
        <span class="mono">{{ audioLine }}</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.info-loading {
  font-size: 12px;
  color: var(--text-mute);
  padding: 6px 0;
}

.info-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 10px;
  margin-bottom: 10px;
}

.cell {
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.cell .k {
  font-size: 11px;
  color: var(--text-mute);
}

.cell .v {
  font-size: 13px;
  font-weight: 500;
}

.streams {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding-top: 10px;
  border-top: 1px solid var(--border);
}

.stream {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--text-dim);
}

.tag {
  flex: none;
  font-size: 11px;
  padding: 1px 6px;
  border-radius: 4px;
  background: #eef1f5;
  color: var(--text-dim);
}
</style>
