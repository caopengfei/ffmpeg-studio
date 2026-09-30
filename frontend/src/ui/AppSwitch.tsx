// HeroUI v3 Switch (compound: Root/Content/Control/Thumb) 封装
import { Switch } from '@heroui/react';

interface AppSwitchProps {
  checked: boolean;
  disabled?: boolean;
  /** 无障碍名称（列表里的开关等多实例场景靠它区分） */
  label?: string;
  onChange: (v: boolean) => void;
}

export default function AppSwitch({ checked, disabled = false, label, onChange }: AppSwitchProps) {
  return (
    <Switch isSelected={checked} isDisabled={disabled} onChange={onChange} aria-label={label}>
      <Switch.Content>
        <Switch.Control>
          <Switch.Thumb />
        </Switch.Control>
      </Switch.Content>
    </Switch>
  );
}
