// HeroUI v3 Popover 外壳 + 原生 <input type="color">：对外 hex #rrggbb（drawtext 兼容）
import { Popover } from '@heroui/react';

interface AppColorProps {
  value: string;
  /** 无障碍名称，同时用于触发器和颜色输入框 */
  label?: string;
  onChange: (hex: string) => void;
}

function normalizeHex(v: string): string {
  const s = (v || '').trim();
  if (/^#[0-9a-fA-F]{6}$/.test(s)) return s.toLowerCase();
  if (/^[0-9a-fA-F]{6}$/.test(s)) return `#${s.toLowerCase()}`;
  const m = /^#([0-9a-fA-F]{3})$/.exec(s);
  if (m) return `#${m[1].split('').map((c) => c + c).join('').toLowerCase()}`;
  return '#ffffff';
}

export default function AppColor({ value, label, onChange }: AppColorProps) {
  const hex = normalizeHex(value);
  return (
    <Popover>
      <Popover.Trigger>
        <span style={{ display: 'inline-flex', alignItems: 'center', gap: 8 }} aria-label={label}>
          <span
            style={{
              width: 20,
              height: 20,
              borderRadius: 4,
              backgroundColor: hex,
              border: '1px solid #555',
              display: 'inline-block',
            }}
          />
          <span style={{ fontFamily: 'monospace' }}>{hex.toUpperCase()}</span>
        </span>
      </Popover.Trigger>
      <Popover.Content>
        <Popover.Dialog aria-label="color picker">
          <input type="color" value={hex} aria-label={label} onChange={(e) => onChange(e.target.value.toLowerCase())} />
        </Popover.Dialog>
      </Popover.Content>
    </Popover>
  );
}
