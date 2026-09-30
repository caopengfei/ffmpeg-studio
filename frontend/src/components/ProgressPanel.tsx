import { useEffect, useRef, useState } from 'react';
import { Accordion, Button, ProgressBar } from '@heroui/react';
import { formatTime, formatSize } from '../api';
import './ProgressPanel.css';

interface ProgressPanelProps {
  running: boolean;
  progress: any;
  stage: any;
  logs: string[];
  result: any;
  onCancel: () => void;
  onOpenFile: (p: string) => void;
  onOpenDir: (p: string) => void;
}

export default function ProgressPanel({
  running,
  // progress 正常由 useTask 保证非空；给默认值防历史调用方透传 undefined 时白屏
  progress = { percent: 0 },
  stage = null,
  logs = [],
  result = null,
  onCancel,
  onOpenFile,
  onOpenDir,
}: ProgressPanelProps) {
  const [showLogs, setShowLogs] = useState(false);
  const prevRunning = useRef(running);

  // 新一轮任务开始（false→true）时收起日志，避免上一轮的展开状态带过来
  useEffect(() => {
    if (running && !prevRunning.current) setShowLogs(false);
    prevRunning.current = running;
  }, [running]);

  // 失败时自动展开日志，省得用户再点一下
  useEffect(() => {
    if (result && !result.ok) setShowLogs(true);
  }, [result]);

  if (!running && !result) return null;

  const pct = Math.max(0, Math.min(100, progress.percent || 0));
  const indeterminate = running && progress.indeterminate;

  let etaText = '';
  {
    const s = progress.etaSec;
    if (s && s > 0) {
      etaText = s < 60 ? `约 ${Math.ceil(s)} 秒` : `约 ${Math.ceil(s / 60)} 分钟`;
    }
  }

  const fillClass = `fill${result?.ok ? ' ok' : result && !result.ok && !result.canceled ? ' err' : ''}`;

  return (
    <div className="panel">
      <div className="head">
        <span className="label">
          {running ? (
            stage ? (
              `[${stage.index}/${stage.total}] ${stage.label}`
            ) : (
              progress.stepLabel || '处理中'
            )
          ) : result?.ok ? (
            '✓ 完成'
          ) : result?.canceled ? (
            '已取消'
          ) : (
            '✗ 失败'
          )}
        </span>

        {running && (
          <span className="stats mono">
            {!indeterminate && <span>{pct.toFixed(1)}%</span>}
            {progress.speed && progress.speed !== 'N/A' && <span>· {progress.speed}</span>}
            {etaText && <span>· 剩余 {etaText}</span>}
          </span>
        )}
      </div>

      <ProgressBar
        className={`bar${indeterminate ? ' indet' : ''}`}
        value={indeterminate ? undefined : pct}
        minValue={0}
        maxValue={100}
        isIndeterminate={indeterminate}
        aria-label="任务进度"
      >
        <ProgressBar.Track className="bartrack">
          <ProgressBar.Fill className={fillClass} />
        </ProgressBar.Track>
      </ProgressBar>

      {running && (
        <div className="meta mono">
          {progress.outTime > 0 && <span>已处理 {formatTime(progress.outTime)}</span>}
          {progress.frame > 0 && <span>{progress.frame} 帧</span>}
          {progress.sizeBytes > 0 && <span>{formatSize(progress.sizeBytes)}</span>}
        </div>
      )}

      {result && (
        <div className={`result ${result.ok ? 'ok' : result.canceled ? 'warn' : 'err'}`}>
          {result.ok ? (
            <>
              输出：{result.outputPath}
              {Number.isFinite(result.elapsedMs) && (
                <span className="dim">（用时 {(result.elapsedMs / 1000).toFixed(1)} 秒）</span>
              )}
            </>
          ) : result.canceled ? (
            '任务已取消，未生成完整文件。'
          ) : (
            result.err || '任务执行失败'
          )}
        </div>
      )}

      <div className="actions">
        {running && (
          <Button size="sm" onPress={onCancel}>
            取消任务
          </Button>
        )}
        {!running && result?.ok && (
          <>
            <Button size="sm" variant="primary" onPress={() => onOpenFile(result.outputPath)}>
              打开文件
            </Button>
            <Button size="sm" onPress={() => onOpenDir(result.outputPath)}>
              打开所在目录
            </Button>
          </>
        )}
        <Accordion.Root
          className="logs-root"
          expandedKeys={showLogs ? ['logs'] : []}
          onExpandedChange={(keys) => setShowLogs(keys.has('logs'))}
        >
          <Accordion.Item id="logs">
            <Accordion.Heading>
              <Accordion.Trigger className="log-toggle">
                {showLogs ? '收起日志' : `日志${logs.length ? `（${logs.length}）` : ''}`}
              </Accordion.Trigger>
            </Accordion.Heading>
            <Accordion.Panel>
              <pre className="logs scrollbox">{logs.join('\n') || '暂无日志'}</pre>
            </Accordion.Panel>
          </Accordion.Item>
        </Accordion.Root>
      </div>
    </div>
  );
}
