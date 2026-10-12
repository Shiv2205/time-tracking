<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api } from '../api'
import { hoursLabel } from '../helpers'
import type { Draft, RemoteLog } from '../types'
import Modal from './Modal.vue'
const props = defineProps<{ entry: Draft }>()
const emit = defineEmits<{ close: []; changed: [] }>()
const candidates = ref<RemoteLog[]>([]),
  checked = ref(false),
  busy = ref(false),
  error = ref(''),
  acknowledged = ref(false)
async function act(action: string, remoteId = '') {
  busy.value = true
  error.value = ''
  try {
    const result = await api<{ candidates: RemoteLog[] }>('/reconcile', 'POST', {
      id: props.entry.id,
      revision: props.entry.revision,
      action,
      remoteId,
    })
    candidates.value = result.candidates
    checked.value = true
    if (action !== 'check') {
      emit('changed')
      emit('close')
    }
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    busy.value = false
  }
}
onMounted(() => act('check'))
</script>
<template>
  <Modal title="Check uncertain submission" :busy="busy" @close="!busy && emit('close')">
    <div class="modal-body">
      <p>{{ entry.taskName }} · {{ hoursLabel(entry.minutes) }} · {{ entry.date }}</p>
      <p class="muted">
        Zoho may have received this entry. It is locked to prevent an accidental duplicate.
      </p>
      <p v-if="busy">Checking fresh Zoho logs…</p>
      <template v-else-if="checked">
        <div v-if="candidates.length === 1" class="notice">
          One matching log found: {{ candidates[0]!.id }}. Inspect it before linking.
        </div>
        <p v-else-if="candidates.length > 1" class="error-message">
          Multiple matches were found. Resolve duplicates in Zoho, then check again.
        </p>
        <p v-else class="notice">
          No exact match was found. Zoho may change notes or take time to expose a new log. Check
          the entry directly in Zoho before releasing it.
        </p>
        <pre v-for="l in candidates" :key="l.id" class="candidate-details"
          >{{ l.date }} · {{ l.taskName }} · {{ hoursLabel(l.minutes) }}
{{ l.notes }}</pre>
        <label v-if="candidates.length < 2" class="checkbox-label confirmation">
          <input v-model="acknowledged" type="checkbox" />
          {{
            candidates.length
              ? 'I inspected the matching log and confirm it is this entry.'
              : 'I checked Zoho directly and confirm this entry was not created.'
          }}
        </label>
        <footer class="modal-footer">
          <button class="button secondary" @click="act('check')">Check again</button>
          <button
            v-if="candidates.length === 1"
            class="button primary"
            :disabled="!acknowledged"
            @click="act('link', candidates[0]!.id)"
          >
            Link Zoho entry
          </button>
          <button
            v-else-if="!candidates.length"
            class="button primary"
            :disabled="!acknowledged"
            @click="act('release')"
          >
            Release for retry
          </button>
        </footer>
      </template>
      <p v-if="error" class="error-message" role="alert">{{ error }}</p>
    </div>
  </Modal>
</template>
