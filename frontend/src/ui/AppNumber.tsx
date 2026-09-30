// HeroUI v3 NumberField (compound) 封装：对外 number；空输入按 0 发出，失焦夹取
// 说明：Vue 版的 echo-back 守卫（输入中不重写输入框）由 RAC NumberField 内部的
// inputValue 状态承担——受控 value 回环为同一数字时不触碰正在编辑的文本；
// 外部传入不同数字时才同步显示。
import { NumberField } from '@heroui/react';

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

const NO_GROUPING: Intl.NumberFormatOptions = { useGrouping: false };

function clamp(v: number, min?: number, max?: number): number {
  let r = v;
  if (min !== undefined) r = Math.max(min, r);
  if (max !== undefined) r = Math.min(max, r);
  return r;
}

export default function AppNumber({ value, min, max, step = 1, disabled = false, placeholder = '', label, onChange }: AppNumberProps) {
  const safe = typeof value === 'number' && !Number.isNaN(value) ? value : 0;
  return (
    <NumberField
      value={safe}
      minValue={min}
      maxValue={max}
      step={step}
      isDisabled={disabled}
      formatOptions={NO_GROUPING}
      aria-label={label}
      onChange={(v) => onChange(Number.isNaN(v) ? 0 : v)}
    >
      <NumberField.Group>
        <NumberField.Input
          placeholder={placeholder}
          onBlur={() => {
            const c = clamp(safe, min, max);
            if (c !== safe) onChange(c);
          }}
        />
        <NumberField.IncrementButton>+</NumberField.IncrementButton>
        <NumberField.DecrementButton>-</NumberField.DecrementButton>
      </NumberField.Group>
    </NumberField>
  );
}
