<script setup>
import { computed } from 'vue'
import { SliderRoot, SliderTrack, SliderRange, SliderThumb } from 'reka-ui'

// 单滑块（Reka UI Slider 封装），CRF / 帧率 / 不透明度这类单值拖杆。
// ⚠️ Reka 运行时只认数组形式的 modelValue（非数组内部按 [] 处理，值恒为 0），
// 所以这里对外的 number 与内部的 [number] 做一层转换
const props = defineProps({
  modelValue: { type: Number, default: 0 },
  min: { type: Number, default: 0 },
  max: { type: Number, default: 100 },
  step: { type: Number, default: 1 },
  disabled: { type: Boolean, default: false },
})
const emit = defineEmits(['update:modelValue'])

const arrValue = computed(() => [props.modelValue])

function onUpdate(v) {
  const n = Array.isArray(v) ? v[0] : v
  if (typeof n === 'number') emit('update:modelValue', n)
}
</script>

<template>
  <SliderRoot
    class="u-slider"
    :model-value="arrValue"
    :min="min"
    :max="max"
    :step="step"
    :disabled="disabled"
    @update:model-value="onUpdate"
  >
    <SliderTrack class="u-slider-track">
      <SliderRange class="u-slider-range" />
    </SliderTrack>
    <SliderThumb class="u-slider-thumb" />
  </SliderRoot>
</template>
