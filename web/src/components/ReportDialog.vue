<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { api } from '../api'
import type { Total } from '../types'
import Modal from './Modal.vue'
import Icon from './Icon.vue'
const props = defineProps<{ projects: Total[]; start: string; initialProject?: string }>()
const emit = defineEmits<{ close: [] }>()
const projectId = ref(props.initialProject || props.projects[0]?.key || ''),
  text = ref(''),
  error = ref(''),
  busy = ref(false),
  copied = ref(false)
const reportArea = ref<HTMLTextAreaElement>()
let generation = 0
async function load() {
  const request = ++generation
  busy.value = true
  error.value = ''
  copied.value = false
  text.value = ''
  try {
    const result = await api<{ text: string }>(
      `/report?start=${props.start}&projectId=${encodeURIComponent(projectId.value)}`,
    )
    if (request === generation) text.value = result.text
  } catch (e) {
    if (request === generation) error.value = (e as Error).message
  } finally {
    if (request === generation) busy.value = false
  }
}
async function copy() {
  try {
    await navigator.clipboard.writeText(text.value)
    copied.value = true
  } catch {
    reportArea.value?.focus()
    reportArea.value?.select()
    error.value = 'The report is selected. Press Ctrl+C or Command+C to copy.'
  }
}
watch(projectId, load)
onMounted(load)
</script>
<template>
  <Modal title="Weekly project report" wide @close="emit('close')">
    <div class="modal-body">
      <p class="muted">A ready-to-share report of your local drafts, with your work notes.</p>
      <label>
        Project
        <select v-model="projectId" aria-label="Project">
          <option v-for="p in projects" :key="p.key" :value="p.key">{{ p.name }}</option>
        </select>
      </label>
      <label class="report-label">
        Report preview
        <textarea
          ref="reportArea"
          aria-label="Report preview"
          class="report-preview"
          :value="busy ? 'Preparing report…' : text"
          readonly
          rows="13"
        />
      </label>
      <p v-if="error" class="error-message" role="alert">{{ error }}</p>
      <footer class="modal-footer">
        <span class="muted small">Draft entries only</span>
        <button class="button primary" :disabled="busy || !text" @click="copy">
          <Icon :name="copied ? 'check' : 'copy'" :size="17" />
          {{ copied ? 'Copied' : 'Copy report' }}
        </button>
      </footer>
    </div>
  </Modal>
</template>
