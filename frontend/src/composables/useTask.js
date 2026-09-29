import { ref, onUnmounted } from 'vue'
import { api, events } from '../api'

// 任务状态：进度、日志、阶段、结果。
// 七个 Tab 各自持有一份，互不干扰。
export function useTask() {
  const running = ref(false)
  const progress = ref({
    percent: 0,
    stepIndex: 0,
    stepTotal: 1,
    stepLabel: '',
    speed: '',
    etaSec: 0,
    frame: 0,
    sizeBytes: 0,
    indeterminate: false,
  })
  const stage = ref(null)
  const logs = ref([])
  const result = ref(null)

  const offs = [
    events.EventsOn('task:progress', (p) => {
      progress.value = p
    }),
    events.EventsOn('task:log', (d) => {
      logs.value.push(d.line)
      // 日志只保留最近 400 行，避免长时间任务把内存吃满
      if (logs.value.length > 400) logs.value.splice(0, logs.value.length - 400)
    }),
    events.EventsOn('task:stage', (s) => {
      stage.value = s
    }),
    events.EventsOn('task:done', (d) => {
      running.value = false
      result.value = d
    }),
  ]

  onUnmounted(() => {
    offs.forEach((off) => {
      if (typeof off === 'function') off()
    })
  })

  function reset() {
    progress.value = {
      percent: 0,
      stepIndex: 0,
      stepTotal: 1,
      stepLabel: '',
      speed: '',
      etaSec: 0,
      frame: 0,
      sizeBytes: 0,
      indeterminate: false,
    }
    stage.value = null
    logs.value = []
    result.value = null
  }

  async function start(spec) {
    reset()
    running.value = true
    try {
      await api.startTask(spec)
    } catch (e) {
      running.value = false
      throw e
    }
  }

  async function cancel() {
    try {
      await api.cancelTask('')
    } catch (e) {
      // 任务可能刚好自己结束了，忽略
    }
  }

  return { running, progress, stage, logs, result, start, cancel, reset }
}
