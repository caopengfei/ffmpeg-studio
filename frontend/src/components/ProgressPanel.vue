<script setup>
import { ref, computed, watch } from 'vue'
import { ProgressRoot, ProgressIndicator, CollapsibleRoot, CollapsibleTrigger, CollapsibleContent } from 'reka-ui'
import { formatTime, formatSize } from '../api'

const props = defineProps({
  running: { type: Boolean, default: false },
  progress: { type: Object, default: () => ({ percent: 0 }) },
  stage: { type: Object, default: null },
  logs: { type: Array, default: () => [] },
  result: { type: Object, default: null },
})
const emit = defineEmits(['cancel'])

const showLogs = ref(false)

watch(
  () => props.result,
  (r) => {
    // 失败时自动展开日志，省得用户再点一下
    if (r && !r.ok) showLogs.value = true
  }
)

// 进度条动画：不确定进度的步骤用条纹提示
const pct = computed(() => Math.max(0, Math.min(100, props.progress.percent || 0)))
const indeterminate = computed(() => props.running && props.progress.indeterminate)

const etaText = computed(() => {
  const s = props.progress.etaSec
  if (!s || s <= 0) return ''
  if (s < 60) return `约 ${Math.ceil(s)} 秒`
  return `约 ${Math.ceil(s / 60)} 分钟`
})
</script>

<template>
  <div v-if="running || result" class="panel">
    <div class="head">
      <span class="label">
        <template v-if="running">
          {{ stage ? `[${stage.index}/${stage.total}] ${stage.label}` : progress.stepLabel || '处理中' }}
        </template>
        <template v-else-if="result?.ok">✓ 完成</template>
        <template v-else-if="result?.canceled">已取消</template>
        <template v-else>✗ 失败</template>
      </span>

      <span v-if="running" class="stats mono">
        <span v-if="!indeterminate">{{ pct.toFixed(1) }}%</span>
        <span v-if="progress.speed && progress.speed !== 'N/A'">· {{ progress.speed }}</span>
        <span v-if="etaText">· 剩余 {{ etaText }}</span>
      </span>
    </div>

    <!-- indeterminate 时传 null，Reka 会带上 data-state=indeterminate -->
    <ProgressRoot :model-value="indeterminate ? null : pct" class="bar" :class="{ indet: indeterminate }">
      <ProgressIndicator
        class="fill"
        :class="{ ok: result?.ok, err: result && !result.ok && !result.canceled }"
        :style="{ width: (indeterminate ? 100 : pct) + '%' }"
      />
    </ProgressRoot>

    <div v-if="running" class="meta mono">
      <span v-if="progress.outTime > 0">已处理 {{ formatTime(progress.outTime) }}</span>
      <span v-if="progress.frame > 0">{{ progress.frame }} 帧</span>
      <span v-if="progress.sizeBytes > 0">{{ formatSize(progress.sizeBytes) }}</span>
    </div>

    <div v-if="result" class="result" :class="result.ok ? 'ok' : result.canceled ? 'warn' : 'err'">
      <template v-if="result.ok">
        输出：{{ result.outputPath }}<span class="dim">（用时 {{ (result.elapsedMs / 1000).toFixed(1) }} 秒）</span>
      </template>
      <template v-else-if="result.canceled">任务已取消，未生成完整文件。</template>
      <template v-else>{{ result.err || '任务执行失败' }}</template>
    </div>

    <div class="actions">
      <button v-if="running" class="tiny" @click="emit('cancel')">取消任务</button>
      <template v-else-if="result?.ok">
        <button class="tiny primary" @click="emit('openFile', result.outputPath)">打开文件</button>
        <button class="tiny" @click="emit('openDir', result.outputPath)">打开所在目录</button>
      </template>
      <CollapsibleRoot v-model:open="showLogs" class="logs-root">
        <CollapsibleTrigger class="tiny ghost log-toggle">
          {{ showLogs ? '收起日志' : `日志${logs.length ? `（${logs.length}）` : ''}` }}
        </CollapsibleTrigger>
        <CollapsibleContent class="u-coll-content">
          <pre class="logs scrollbox">{{ logs.join('\n') || '暂无日志' }}</pre>
        </CollapsibleContent>
      </CollapsibleRoot>
    </div>
  </div>
</template>

<style scoped>
.panel {
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--panel);
  padding: 12px 14px;
  margin-bottom: 14px;
}

.head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
}

.label {
  font-size: 13px;
  font-weight: 500;
}

.stats {
  font-size: 11.5px;
  color: var(--text-dim);
}

.bar {
  display: block;
  height: 7px;
  border-radius: 4px;
  background: #eef1f5;
  overflow: hidden;
}

.fill {
  display: block;
  height: 100%;
  background: var(--primary);
  border-radius: 4px;
  transition: width 0.25s ease;
}

.fill.ok {
  background: #16a34a;
}

.fill.err {
  background: #dc2626;
}

.bar.indet .fill {
  background: linear-gradient(90deg, #93b4f7 0%, #2563eb 50%, #93b4f7 100%);
  background-size: 200% 100%;
  animation: indet-slide 1.1s linear infinite;
  opacity: 0.55;
}

@keyframes indet-slide {
  from {
    background-position: 100% 0;
  }
  to {
    background-position: -100% 0;
  }
}

.meta {
  display: flex;
  gap: 14px;
  margin-top: 7px;
  font-size: 11.5px;
  color: var(--text-mute);
}

.result {
  margin-top: 10px;
  padding: 9px 11px;
  border-radius: var(--radius-sm);
  font-size: 12px;
  line-height: 1.6;
  word-break: break-all;
}

.result.ok {
  background: var(--ok-soft);
  border: 1px solid #bbf7d0;
  color: var(--ok);
}

.result.warn {
  background: var(--warn-soft);
  border: 1px solid #fde68a;
  color: var(--warn);
}

.result.err {
  background: var(--err-soft);
  border: 1px solid #fecaca;
  color: var(--err);
}

.result .dim {
  color: var(--text-mute);
}

.actions {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  margin-top: 10px;
  flex-wrap: wrap;
}

.logs-root {
  display: flex;
  flex-direction: column;
  flex: 1;
}

.log-toggle {
  border: 1px solid var(--border);
  background: #fff;
  color: var(--text-dim);
  align-self: flex-start;
}

.log-toggle:hover:not(:disabled) {
  border-color: var(--border-strong);
  background: #f7f9fc;
}

.logs {
  margin: 10px 0 0;
  padding: 10px 12px;
  max-height: 200px;
  background: #f7f8fa;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  font-size: 11px;
  line-height: 1.55;
  color: #475569;
  white-space: pre-wrap;
  word-break: break-all;
  user-select: text;
}
</style>
