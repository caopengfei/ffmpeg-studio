import { useEffect, useRef, useState } from 'react';
import { Accordion, Button } from '@heroui/react';
import './CommandPreview.css';

interface CommandPreviewProps {
  steps: string[];
  err: string;
}

export default function CommandPreview({ steps, err }: CommandPreviewProps) {
  const [copied, setCopied] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  // 卸载时清掉"已复制"回弹定时器，避免在已卸载组件上 setState
  useEffect(
    () => () => {
      if (timer.current) clearTimeout(timer.current);
    },
    []
  );

  async function copy() {
    const text = steps.join('\n');
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      if (timer.current) clearTimeout(timer.current);
      timer.current = setTimeout(() => setCopied(false), 1500);
    } catch {
      /* 剪贴板不可用时忽略 */
    }
  }

  return (
    <div className="cmd">
      <Accordion.Root className="cmd-root">
        <Accordion.Item id="cmd">
          <Accordion.Heading>
            <Accordion.Trigger className="toggle">
              <Accordion.Indicator>
                <span className="caret">›</span>
              </Accordion.Indicator>
              将执行的命令
              {steps.length > 1 && <span className="badge">{steps.length} 步</span>}
            </Accordion.Trigger>
          </Accordion.Heading>
          <Accordion.Panel>
            <div className="body">
              {err ? (
                <div className="banner err">{err}</div>
              ) : steps.length ? (
                <>
                  <div className="toolbar">
                    <Button size="sm" onPress={copy}>
                      {copied ? '已复制' : '复制'}
                    </Button>
                  </div>
                  {steps.map((s, i) => (
                    <pre key={i} className="line">
                      {s}
                    </pre>
                  ))}
                </>
              ) : (
                <div className="empty-hint">参数填完整后这里会显示要执行的命令</div>
              )}
            </div>
          </Accordion.Panel>
        </Accordion.Item>
      </Accordion.Root>
    </div>
  );
}
