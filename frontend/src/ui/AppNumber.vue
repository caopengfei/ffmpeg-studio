<script setup>
import { ref, watch } from 'vue'
import {
  NumberInputRoot,
  NumberInputInput,
  NumberInputControl,
  NumberInputIncrementTrigger,
  NumberInputDecrementTrigger,
} from '@ark-ui/vue'

// 数字输入（Ark UI NumberInput 封装），带步进按钮、上下键增减、失焦夹取。
// Ark 的 v-model 是字符串，这里对外统一为 number
const props = defineProps({
  modelValue: { type: Number, default: 0 },
  min: { type: Number, default: undefined },
  max: { type: Number, default: undefined },
  step: { type: Number, default: 1 },
  disabled: { type: Boolean, default: false },
  placeholder: { type: String, default: '' },
})
const emit = defineEmits(['update:modelValue'])

const sv = ref(props.modelValue == null || Number.isNaN(props.modelValue) ? '' : String(props.modelValue))

watch(
  () => props.modelValue,
  (v) => {
    // 自己 emit 出去的值回环时不要重写输入框，避免打断输入
    if (v !== Number(sv.value)) sv.value = v == null || Number.isNaN(v) ? '' : String(v)
  }
)

function onInput(v) {
  sv.value = v ?? ''
  const n = Number(sv.value)
  emit('update:modelValue', sv.value === '' || Number.isNaN(n) ? 0 : n)
}
</script>

<template>
  <NumberInputRoot
    class="u-num"
    :model-value="sv"
    :min="min"
    :max="max"
    :step="step"
    :disabled="disabled"
    @update:model-value="onInput"
  >
    <NumberInputInput class="u-num-input" :placeholder="placeholder" />
    <NumberInputControl class="u-num-ctl">
      <NumberInputIncrementTrigger class="u-num-btn" title="增加">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round">
          <path d="m6 15 6-6 6 6" />
        </svg>
      </NumberInputIncrementTrigger>
      <NumberInputDecrementTrigger class="u-num-btn" title="减少">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round">
          <path d="m6 9 6 6 6-6" />
        </svg>
      </NumberInputDecrementTrigger>
    </NumberInputControl>
  </NumberInputRoot>
</template>
