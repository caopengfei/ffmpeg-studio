// 纯数字输入框（无 +/− 步进按钮）。
//
// 不用 HeroUI NumberField：它的组是 :has(slot) 切出的 40px|1fr|40px 三列，
// 两颗步进按钮夹在输入框两侧显得笨重（宽 60~1280 这类精调参数用不上步进，
// 还容易误点）。这里改用单个 Input，保留 NumberField 时期的三件事：
//   - 空输入按 0 发出
//   - 失焦按 min/max 夹取并回写
//   - 输入过程中不回写（本地草稿态），外部值变化才同步
import { Input } from '@heroui/react';
import { useEffect, useRef, useState } from 'react';

interface AppNumberProps {
  value: number;
  min?: number;
  max?: number;
  step?: number;
  disabled?: boolean;
  placeholder?: string;
  /** 无障碍名称 */
  label?: string;
  onChange: (v: number) => void;
}

function clamp(v: number, min?: number, max?: number): number {
  let r = v;
  if (min !== undefined) r = Math.max(min, r);
  if (max !== undefined) r = Math.min(max, r);
  return r;
}

export default function AppNumber({ value, min, max, step = 1, disabled = false, placeholder = '', label, onChange }: AppNumberProps) {
  const safe = typeof value === 'number' && !Number.isNaN(value) ? value : 0;
  // 草稿态：用户正在敲的内容与外部 value 不一定一致，直接受控会把输入打断
  const [draft, setDraft] = useState(String(safe));
  const editing = useRef(false);

  // 外部值变了才同步显示；自己刚 emit 出去的值回环时不重写输入框
  useEffect(() => {
    if (editing.current) return;
    setDraft(String(safe));
  }, [safe]);

  function commit(text: string) {
    const n = Number(text);
    return text === '' || Number.isNaN(n) ? 0 : n;
  }

  return (
    <Input
      className="app-number"
      // 数字框用 inputMode 而非 type=number：后者有原生步进箭头，
      // 滚轮又会误改数值，中文输入法还会顶出上下箭头
      type="text"
      inputMode="decimal"
      autoComplete="off"
      value={draft}
      placeholder={placeholder}
      disabled={disabled}
      aria-label={label}
      onFocus={() => {
        editing.current = true;
      }}
      onChange={(e) => {
        const text = e.target.value;
        // 只留数字与小数点，第二个及之后的小数点连同其后的内容一并截掉
        // （用正则整体匹配会误伤 "2."：回溯时空捕获组可为空，"2." 会被清成空串）
        const parts = text.replace(/[^\d.]/g, '').split('.');
        const cleaned = parts.length > 2 ? `${parts[0]}.${parts[1]}` : parts.join('.');
        setDraft(cleaned);
        onChange(commit(cleaned));
      }}
      onBlur={() => {
        editing.current = false;
        const c = clamp(commit(draft), min, max);
        setDraft(String(c));
        if (c !== safe) onChange(c);
      }}
      onKeyDown={(e) => {
        // 上下键按 step 微调，配合 min/max 夹取
        if (e.key !== 'ArrowUp' && e.key !== 'ArrowDown') return;
        e.preventDefault();
        const dir = e.key === 'ArrowUp' ? 1 : -1;
        const n = clamp(commit(draft) + dir * (step || 1), min, max);
        setDraft(String(n));
        onChange(n);
      }}
    />
  );
}
