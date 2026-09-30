<script setup>
import { computed } from 'vue'
import { SwitchRoot, SwitchThumb } from 'reka-ui'

// 开关（Reka UI Switch 封装），水印列表的启用/禁用用
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
  <SwitchRoot v-model="model" class="u-switch" :disabled="disabled">
    <SwitchThumb class="u-switch-thumb" />
  </SwitchRoot>
</template>
