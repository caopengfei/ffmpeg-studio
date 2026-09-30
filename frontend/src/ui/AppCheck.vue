<script setup>
import { computed } from 'vue'
import { CheckboxRoot, CheckboxIndicator } from 'reka-ui'

// 复选框（Reka UI Checkbox 封装），标签文字走 slot
const props = defineProps({
  modelValue: { type: Boolean, default: false },
  disabled: { type: Boolean, default: false },
})
const emit = defineEmits(['update:modelValue'])
const model = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v === true),
})
</script>

<template>
  <label class="check" :class="{ disabled }">
    <CheckboxRoot v-model="model" class="u-check" :disabled="disabled">
      <CheckboxIndicator class="u-check-ind">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round">
          <path d="m5 12.5 4.5 4.5L19 7" />
        </svg>
      </CheckboxIndicator>
    </CheckboxRoot>
    <slot />
  </label>
</template>
