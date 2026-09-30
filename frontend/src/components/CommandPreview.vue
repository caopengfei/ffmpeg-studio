<script setup>
import { ref } from 'vue'
import { CollapsibleRoot, CollapsibleTrigger, CollapsibleContent } from 'reka-ui'

const props = defineProps({
  steps: { type: Array, default: () => [] },
  err: { type: String, default: '' },
})

const copied = ref(false)

async function copy() {
  const text = props.steps.join('\n')
  try {
    await navigator.clipboard.writeText(text)
    copied.value = true
    setTimeout(() => (copied.value = false), 1500)
  } catch {
    /* 剪贴板不可用时忽略 */
  }
}
</script>

<template>
  <div class="cmd">
    <CollapsibleRoot class="cmd-root">
      <CollapsibleTrigger class="toggle ghost">
        <span class="caret">›</span>
        将执行的命令
        <span v-if="steps.length > 1" class="badge">{{ steps.length }} 步</span>
      </CollapsibleTrigger>

      <CollapsibleContent class="u-coll-content">
        <div class="body">
          <div v-if="err" class="banner err">{{ err }}</div>

          <template v-else-if="steps.length">
            <div class="toolbar">
              <button class="tiny" @click="copy">{{ copied ? '已复制' : '复制' }}</button>
            </div>
            <pre v-for="(s, i) in steps" :key="i" class="line">{{ s }}</pre>
          </template>

          <div v-else class="empty-hint">参数填完整后这里会显示要执行的命令</div>
        </div>
      </CollapsibleContent>
    </CollapsibleRoot>
  </div>
</template>

<style scoped>
.cmd {
  margin-top: 2px;
}

.toggle {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  border: none;
  background: none;
  padding: 5px 8px;
}

.caret {
  display: inline-block;
  transition: transform 0.15s;
  font-size: 14px;
  line-height: 1;
}

.toggle[data-state='open'] .caret {
  transform: rotate(90deg);
}

.badge {
  font-size: 11px;
  padding: 0 6px;
  border-radius: 8px;
  background: #eef1f5;
  color: var(--text-dim);
}

.body {
  margin-top: 8px;
}

.toolbar {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 6px;
}

.line {
  margin: 0 0 6px;
  padding: 9px 11px;
  background: #f7f8fa;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  font-size: 11.5px;
  line-height: 1.55;
  color: #334155;
  white-space: pre-wrap;
  word-break: break-all;
  user-select: text;
}
</style>
