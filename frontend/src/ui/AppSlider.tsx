// HeroUI v3 Slider (compound) 封装：对外单个 number（RAC 底层接受 number | number[]，此处归一为 number）
import { Slider } from '@heroui/react';

interface AppSliderProps {
  value: number;
  min: number;
  max: number;
  step: number;
  disabled?: boolean;
  onChange: (v: number) => void;
}

export default function AppSlider({ value, min, max, step, disabled = false, onChange }: AppSliderProps) {
  return (
    <Slider
      value={value}
      minValue={min}
      maxValue={max}
      step={step}
      isDisabled={disabled}
      aria-label="slider"
      onChange={(v) => onChange(typeof v === 'number' ? v : (v[0] ?? value))}
    >
      <Slider.Track>
        <Slider.Fill />
        <Slider.Thumb />
      </Slider.Track>
    </Slider>
  );
}
