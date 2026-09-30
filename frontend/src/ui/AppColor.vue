<script setup>
import { ref, computed, watch } from 'vue'
import {
  ColorPickerRoot,
  ColorPickerTrigger,
  ColorPickerPositioner,
  ColorPickerContent,
  ColorPickerArea,
  ColorPickerAreaBackground,
  ColorPickerAreaThumb,
  ColorPickerChannelSlider,
  ColorPickerChannelSliderTrack,
  ColorPickerChannelSliderThumb,
  ColorPickerValueSwatch,
} from '@ark-ui/vue'
import { parseColor } from '@zag-js/color-utils'

// 取色器（Ark UI ColorPicker 封装）：触发器显示色块 + hex，弹层是饱和度面 + 色相条。
// 对外统一 hex 字符串（#rrggbb），与 ffmpeg drawtext 颜色直接兼容
const props = defineProps({
  modelValue: { type: String, default: '#ffffff' },
})
const emit = defineEmits(['update:modelValue'])

const hex = computed(() => props.modelValue || '#ffffff')
const color = ref(parseColor(hex.value))

watch(hex, (h) => {
  const c = parseColor(h)
  if (c.toString('hex') !== color.value.toString('hex')) color.value = c
})

function onUpdate(c) {
  color.value = c
  emit('update:modelValue', c.toString('hex'))
}
</script>

<template>
  <ColorPickerRoot
    class="u-color"
    :model-value="color"
    :positioning="{ placement: 'bottom-start', gutter: 6 }"
    @update:model-value="onUpdate"
  >
    <ColorPickerTrigger class="u-color-trigger">
      <ColorPickerValueSwatch class="u-color-swatch" />
      <span class="u-color-hex mono">{{ hex }}</span>
    </ColorPickerTrigger>

    <ColorPickerPositioner>
      <ColorPickerContent class="u-color-pop">
        <ColorPickerArea class="u-color-area">
          <ColorPickerAreaBackground />
          <ColorPickerAreaThumb class="u-color-area-thumb" />
        </ColorPickerArea>
        <ColorPickerChannelSlider channel="hue" class="u-color-hue">
          <ColorPickerChannelSliderTrack class="u-color-hue-track">
            <ColorPickerChannelSliderThumb class="u-color-hue-thumb" />
          </ColorPickerChannelSliderTrack>
        </ColorPickerChannelSlider>
      </ColorPickerContent>
    </ColorPickerPositioner>
  </ColorPickerRoot>
</template>
