<script setup lang="ts">
import { computed, ref } from 'vue'
import { api } from '../api'
import { hoursLabel, shortDate } from '../helpers'
import type { Draft } from '../types'
import Modal from './Modal.vue'
import Icon from './Icon.vue'
const props = defineProps<{ entries: Draft[]; demo: boolean }>()
const emit = defineEmits<{ close: []; submitted: [message: string] }>()
const selected = ref(props.entries.map((e) => e.id)),
  confirmed = ref(false),
  busy = ref(false),
  error = ref('')
const chosen = computed(() => props.entries.filter((e) => selected.value.includes(e.id)))
const total = computed(() => chosen.value.reduce((sum, e) => sum + e.minutes, 0))
async function submit() {
  if (!confirmed.value || !chosen.value.length) return
  busy.value = true
  error.value = ''
  try {
    const result = await api<{ entries: Draft[] }>('/submissions', 'POST', {
      entries: chosen.value.map((e) => ({ id: e.id, revision: e.revision })),
    })
    const count = result.entries.filter((e) => e.status === 'submitted').length
    const issues = result.entries.filter((e) => e.status !== 'submitted').length
    emit(
      'submitted',
      `${count} ${props.demo ? 'simulated' : 'confirmed'} submission${count === 1 ? '' : 's'}${issues ? `; ${issues} need attention` : ''}.`,
    )
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    busy.value = false
  }
}
</script>
<template>
  <Modal title="Review your time" wide :busy="busy" @close="!busy && emit('close')">
    <div class="modal-body">
      <p class="muted">
        Choose the drafts to send. Existing Zoho logs are excluded from this review.
      </p>
      <div class="review-list">
        <label v-for="e in entries" :key="e.id" class="review-row">
          <input v-model="selected" type="checkbox" :value="e.id" :disabled="busy" />
          <div>
            <strong>{{ e.taskName }}</strong>
            <span>{{ e.projectName }} · {{ shortDate(e.date) }}</span>
            <p v-if="e.notes">{{ e.notes }}</p>
          </div>
          <b>{{ hoursLabel(e.minutes) }}</b>
        </label>
      </div>
      <div class="review-total">
        <span>{{ chosen.length }} selected {{ chosen.length === 1 ? 'entry' : 'entries' }}</span>
        <strong>{{ hoursLabel(total) }}</strong>
      </div>
      <div class="notice">
        <Icon name="info" :size="18" />
        <span>Copy your project reports before submitting. Reports contain drafts only.</span>
      </div>
      <label class="checkbox-label confirmation">
        <input v-model="confirmed" type="checkbox" :disabled="busy" />
        {{
          demo
            ? 'Simulate sending these entries in demo mode.'
            : 'I reviewed these entries and want to send them to Zoho.'
        }}
      </label>
      <p v-if="chosen.length > 50" class="error-message">Submit up to 50 entries at a time.</p>
      <p v-if="error" class="error-message" role="alert">{{ error }}</p>
      <footer class="modal-footer">
        <button class="button secondary" :disabled="busy" @click="emit('close')">
          Keep editing
        </button>
        <button
          class="button primary"
          :disabled="busy || !confirmed || !chosen.length || chosen.length > 50"
          @click="submit"
        >
          {{ busy ? 'Submitting…' : demo ? 'Simulate submission' : 'Submit to Zoho' }}
          <Icon name="arrow" :size="17" />
        </button>
      </footer>
    </div>
  </Modal>
</template>
