<script setup>
import { computed } from 'vue'
import {
  SelectRoot,
  SelectTrigger,
  SelectValue,
  SelectIcon,
  SelectPortal,
  SelectContent,
  SelectViewport,
  SelectItem,
  SelectItemText,
  SelectItemIndicator,
} from 'reka-ui'

// 下拉选择（Reka UI Select 封装）。
// 统一选项格式 { value, label }；value 统一转成字符串（Reka 要求），
// 因此绑定值一律按字符串处理，业务侧需要数字时自己 Number() 转换
const props = defineProps({
  modelValue: { type: [String, Number], default: '' },
  options: { type: Array, default: () => [] }, // [{ value, label, disabled? }]
  placeholder: { type: String, default: '请选择' },
  disabled: { type: Boolean, default: false },
})
const emit = defineEmits(['update:modelValue'])

const normalized = computed(() =>
  props.options.map((o) => ({
    value: String(o.value),
    label: o.label ?? String(o.value),
    disabled: !!o.disabled,
  }))
)

const val = computed({
  get: () => String(props.modelValue ?? ''),
  set: (v) => emit('update:modelValue', v),
})
</script>

<template>
  <SelectRoot v-model="val" :disabled="disabled">
    <SelectTrigger class="u-sel" :aria-label="placeholder">
      <SelectValue :placeholder="placeholder" />
      <SelectIcon class="u-sel-ico">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="m6 9 6 6 6-6" />
        </svg>
      </SelectIcon>
    </SelectTrigger>

    <SelectPortal>
      <SelectContent class="u-sel-pop" position="popper" :side-offset="4" align="start">
        <SelectViewport class="u-sel-viewport">
          <SelectItem v-for="o in normalized" :key="o.value" :value="o.value" :disabled="o.disabled" class="u-sel-item">
            <SelectItemText>{{ o.label }}</SelectItemText>
            <SelectItemIndicator class="u-sel-ind">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round">
                <path d="m5 12.5 4.5 4.5L19 7.5" />
              </svg>
            </SelectItemIndicator>
          </SelectItem>
        </SelectViewport>
      </SelectContent>
    </SelectPortal>
  </SelectRoot>
</template>
