import { useEffect, useState } from 'react'
import { api, events, TaskProgress } from '../api'

const emptyProgress: TaskProgress = {
  percent: 0, stepIndex: 0, stepTotal: 1, stepLabel: '',
  speed: '', etaSec: 0, frame: 0, sizeBytes: 0, indeterminate: false,
}

// 由 useTask.js 平移：七个 Tab 各自持有一份；事件在 useEffect 订阅、return 清理；日志截 400 行。
export function useTask() {
  const [running, setRunning] = useState(false)
  const [progress, setProgress] = useState<TaskProgress>(emptyProgress)
  const [stage, setStage] = useState<any>(null)
  const [logs, setLogs] = useState<string[]>([])
  const [result, setResult] = useState<any>(null)

  useEffect(() => {
    const offs = [
      events.EventsOn('task:progress', (p: TaskProgress) => setProgress(p)),
      events.EventsOn('task:log', (d: any) => {
        setLogs((prev) => {
          const next = [...prev, d.line]
          return next.length > 400 ? next.slice(next.length - 400) : next
        })
      }),
      events.EventsOn('task:stage', (s: any) => setStage(s)),
      events.EventsOn('task:done', (d: any) => { setRunning(false); setResult(d) }),
    ]
    return () => { offs.forEach((off) => { if (typeof off === 'function') off() }) }
  }, [])

  function reset() {
    setProgress(emptyProgress); setStage(null); setLogs([]); setResult(null)
  }

  async function start(spec: any) {
    reset()
    setRunning(true)
    try {
      await api.startTask(spec)
    } catch (e) {
      setRunning(false)
      throw e
    }
  }

  async function cancel() {
    try { await api.cancelTask('') } catch { /* 任务可能刚好自己结束，忽略 */ }
  }

  return { running, progress, stage, logs, result, start, cancel, reset }
}
