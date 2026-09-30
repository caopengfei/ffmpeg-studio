<script setup>
import { ref, onUnmounted } from 'vue'
import { motion, AnimatePresence } from 'motion-v'
import { api, events, formatSize } from '../api'

const props = defineProps({
  modelValue: { type: Object, default: null },
  accept: { type: String, default: 'media' }, // media | image | font
  label: { type: String, default: '' },
})
const emit = defineEmits(['update:modelValue', 'changed'])

const hovering = ref(false)

async function choose() {
  let f = null
  if (props.accept === 'image') f = await api.pickImageFile()
  else if (props.accept === 'font') f = await api.pickFontFile()
  else f = await api.pickMediaFile()

  if (f) {
    emit('update:modelValue', f)
    emit('changed', f)
  }
}

// 窗口拖放：把文件直接拖进来也能用
const off = events.OnFileDrop((x, y, paths) => {
  hovering.value = false
  if (!paths || !paths.length) return
  const f = api.registerFile(paths[0])
  emit('update:modelValue', f)
  emit('changed', f)
})

onUnmounted(() => {
  if (typeof off === 'function') off()
  events.OnFileDropOff?.()
})
</script>

<template>
  <div
    class="picker"
    :class="{ hovering }"
    @dragenter.prevent="hovering = true"
    @dragover.prevent
    @dragleave.prevent="hovering = false"
  >
    <AnimatePresence mode="wait" :initial="false">
      <motion.div
        v-if="!modelValue"
        key="empty"
        class="pick-empty"
        :initial="{ opacity: 0, y: -4 }"
        :animate="{ opacity: 1, y: 0 }"
        :exit="{ opacity: 0, y: 4 }"
        :transition="{ duration: 0.14 }"
      >
        <div class="pick-hint">{{ label || '选择文件，或直接拖进来' }}</div>
        <button class="primary" @click="choose">浏览…</button>
      </motion.div>

      <motion.div
        v-else
        key="filled"
        class="pick-filled"
        :initial="{ opacity: 0, y: -4 }"
        :animate="{ opacity: 1, y: 0 }"
        :exit="{ opacity: 0, y: 4 }"
        :transition="{ duration: 0.14 }"
      >
        <div class="pick-info">
          <div class="pick-name" :title="modelValue.path">{{ modelValue.name }}</div>
          <div class="pick-meta mono">{{ formatSize(modelValue.size) }}</div>
        </div>
        <button class="tiny" @click="choose">更换</button>
      </motion.div>
    </AnimatePresence>
  </div>
</template>

<style scoped>
.picker {
  border: 1px dashed var(--border-strong);
  border-radius: var(--radius-sm);
  background: #fbfcfd;
  transition: border-color 0.15s, background 0.15s;
}

.picker.hovering {
  border-color: var(--primary);
  background: var(--primary-soft);
}

.pick-empty {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 14px;
}

.pick-hint {
  color: var(--text-mute);
  font-size: 12px;
}

.pick-filled {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 9px 12px;
  background: #fff;
  border-radius: var(--radius-sm);
}

.pick-info {
  min-width: 0;
}

.pick-name {
  font-size: 13px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.pick-meta {
  font-size: 11px;
  color: var(--text-mute);
}
</style>
