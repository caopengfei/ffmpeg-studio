// HeroUI v3 Select (compound) 封装：对外 value 统一字符串
import { ListBox, Select } from '@heroui/react';

export interface AppSelectOption {
  value: string | number;
  label: string;
  disabled?: boolean;
}

interface AppSelectProps {
  value: string | number;
  options: AppSelectOption[];
  placeholder?: string;
  disabled?: boolean;
  onChange: (v: string) => void;
}

export default function AppSelect({ value, options, placeholder = '请选择', disabled = false, onChange }: AppSelectProps) {
  const key = String(value ?? '');
  return (
    <Select
      selectedKey={key}
      onSelectionChange={(k) => onChange(String(k))}
      isDisabled={disabled}
      placeholder={placeholder}
      aria-label={placeholder}
    >
      <Select.Trigger>
        <Select.Value />
        <Select.Indicator />
      </Select.Trigger>
      <Select.Popover>
        <ListBox>
          {options.map((o) => {
            const v = String(o.value);
            return (
              <ListBox.Item key={v} id={v} textValue={o.label ?? v} isDisabled={!!o.disabled}>
                {o.label ?? v}
              </ListBox.Item>
            );
          })}
        </ListBox>
      </Select.Popover>
    </Select>
  );
}
