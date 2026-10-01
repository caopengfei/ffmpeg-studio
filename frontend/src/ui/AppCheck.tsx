// HeroUI v3 Checkbox (compound: Root/Content/Control/Indicator) 封装：标签文字走 children
import type { ReactNode } from 'react';
import { Checkbox } from '@heroui/react';

interface AppCheckProps {
  checked: boolean;
  disabled?: boolean;
  onChange: (v: boolean) => void;
  children: ReactNode;
}

export default function AppCheck({ checked, disabled = false, onChange, children }: AppCheckProps) {
  return (
    <Checkbox isSelected={checked} isDisabled={disabled} onChange={onChange}>
      <Checkbox.Content>
        <Checkbox.Control>
          <Checkbox.Indicator />
        </Checkbox.Control>
        {children}
      </Checkbox.Content>
    </Checkbox>
  );
}
