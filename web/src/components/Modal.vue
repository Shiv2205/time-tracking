<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import Icon from './Icon.vue'
defineProps<{ title: string; wide?: boolean; busy?: boolean }>()
const emit = defineEmits<{ close: [] }>()
const panel = ref<HTMLElement>()
let previous: HTMLElement | null = null
function keydown(event: KeyboardEvent) {
  if (event.key === 'Escape') emit('close')
  if (event.key !== 'Tab') return
  const nodes = panel.value?.querySelectorAll<HTMLElement>(
    'button:not(:disabled), input:not(:disabled), select:not(:disabled), textarea:not(:disabled), a[href]',
  )
  if (!nodes?.length) return
  const first = nodes[0]!,
    last = nodes[nodes.length - 1]!
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault()
    last.focus()
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault()
    first.focus()
  }
}
onMounted(() => {
  previous = document.activeElement as HTMLElement
  panel.value?.querySelector<HTMLElement>('button:not(:disabled)')?.focus()
  document.addEventListener('keydown', keydown)
  document.body.style.overflow = 'hidden'
})
onUnmounted(() => {
  document.removeEventListener('keydown', keydown)
  document.body.style.overflow = ''
  previous?.focus()
})
</script>
<template>
  <div class="modal-backdrop" @click.self="!busy && emit('close')">
    <section
      ref="panel"
      class="modal"
      :class="{ wide }"
      role="dialog"
      aria-modal="true"
      :aria-label="title"
      tabindex="-1"
    >
      <header class="modal-header">
        <h2>{{ title }}</h2>
        <button
          class="icon-button"
          aria-label="Close dialog"
          :disabled="busy"
          @click="emit('close')"
        >
          <Icon name="close" />
        </button>
      </header>
      <slot />
    </section>
  </div>
</template>
