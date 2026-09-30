// HeroUI v3 Switch (compound: Root/Content/Control/Thumb) 封装
import { Switch } from '@heroui/react';

interface AppSwitchProps {
  checked: boolean;
  disabled?: boolean;
  onChange: (v: boolean) => void;
}

export default function AppSwitch({ checked, disabled = false, onChange }: AppSwitchProps) {
  return (
    <Switch isSelected={checked} isDisabled={disabled} onChange={onChange}>
      <Switch.Content>
        <Switch.Control>
          <Switch.Thumb />
        </Switch.Control>
      </Switch.Content>
    </Switch>
  );
}
