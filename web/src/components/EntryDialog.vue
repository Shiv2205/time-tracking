<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { api } from '../api'
import { duration, parseDuration, today } from '../helpers'
import type { Catalog, Draft, DraftInput } from '../types'
import Modal from './Modal.vue'
import Icon from './Icon.vue'
const props = defineProps<{
  catalog: Catalog
  entry?: Draft
  initialDate: string
  duplicate?: boolean
}>()
const emit = defineEmits<{ close: []; saved: [] }>()
const form = reactive({
  date: props.entry?.date || props.initialDate || today(),
  projectId: props.entry?.projectId || '',
  taskId: props.entry?.taskId || '',
  time: props.entry ? duration(props.entry.minutes) : '',
  billable: props.entry?.billable || false,
  notes: props.entry?.notes || '',
})
const busy = ref(false),
  error = ref('')
const tasks = computed(() => props.catalog.tasks.filter((t) => t.projectId === form.projectId))
watch(
  () => form.projectId,
  () => {
    form.taskId = ''
  },
)
async function save() {
  error.value = ''
  try {
    const minutes = parseDuration(form.time)
    if (!form.projectId || !form.taskId) throw new Error('Select a project and task.')
    busy.value = true
    const editing = props.entry && !props.duplicate
    const input: DraftInput = {
      date: form.date,
      projectId: form.projectId,
      taskId: form.taskId,
      minutes,
      billable: form.billable,
      notes: form.notes,
      revision: editing ? props.entry!.revision : 0,
    }
    await api(editing ? `/drafts/${props.entry!.id}` : '/drafts', editing ? 'PUT' : 'POST', input)
    emit('saved')
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    busy.value = false
  }
}
</script>
<template>
  <Modal
    :title="entry && !duplicate ? 'Edit time entry' : 'Add time entry'"
    :busy="busy"
    @close="!busy && emit('close')"
  >
    <form class="modal-body entry-form" @submit.prevent="save">
      <p class="muted">Save your work as a local draft. Submit it when your week is ready.</p>
      <div class="form-row">
        <label>
          Date
          <input v-model="form.date" type="date" required />
        </label>
        <label>
          Duration
          <input v-model="form.time" placeholder="1:30" inputmode="decimal" required />
          <small>H:MM or decimal hours (1.5)</small>
        </label>
      </div>
      <label>
        Project
        <select v-model="form.projectId" aria-label="Project" required>
          <option disabled value="">Choose a project</option>
          <option v-for="p in catalog.projects.filter((p) => p.active)" :key="p.id" :value="p.id">
            {{ p.name }}
          </option>
        </select>
      </label>
      <label>
        Task
        <select v-model="form.taskId" aria-label="Task" required :disabled="!form.projectId">
          <option disabled value="">Choose a task</option>
          <option v-for="t in tasks" :key="t.id" :value="t.id">
            {{ t.name }}{{ t.assigned ? ' · assigned to you' : ''
            }}{{ t.closed ? ' · completed' : '' }}
          </option>
        </select>
      </label>
      <label>
        Work notes
        <span class="optional">Optional</span>
        <textarea
          v-model="form.notes"
          aria-label="Work notes"
          rows="4"
          maxlength="10000"
          placeholder="What did you work on? These notes will appear in your project report."
        />
        <small>{{ form.notes.length.toLocaleString() }} / 10,000</small>
      </label>
      <label class="checkbox-label">
        <input v-model="form.billable" type="checkbox" />
        Billable time
      </label>
      <p v-if="error" class="error-message" role="alert">{{ error }}</p>
      <footer class="modal-footer">
        <button type="button" class="button secondary" :disabled="busy" @click="emit('close')">
          Cancel
        </button>
        <button class="button primary" :disabled="busy">
          <Icon name="check" :size="17" />
          {{ busy ? 'Saving…' : 'Save draft' }}
        </button>
      </footer>
    </form>
  </Modal>
</template>
