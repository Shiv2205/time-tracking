<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { api } from '../api'
import { duration, parseDuration, today } from '../helpers'
import {
  formatCommentNotes,
  parseCommentNotes,
  validateCommentSections,
  type CommentSection,
} from '../commentNotes'
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
const originalNotes = props.entry?.notes || ''
const parsedNotes = parseCommentNotes(originalNotes)
const form = reactive({
  date: props.entry?.date || props.initialDate || today(),
  projectId: props.entry?.projectId || '',
  taskId: props.entry?.taskId || '',
  time: props.entry ? duration(props.entry.minutes) : '',
  billable: props.entry?.billable || false,
  rawNotes: parsedNotes.raw,
  sections: parsedNotes.sections,
})
const initialComments = JSON.stringify({ raw: form.rawNotes, sections: form.sections })
const notes = computed(() =>
  JSON.stringify({ raw: form.rawNotes, sections: form.sections }) === initialComments
    ? originalNotes
    : formatCommentNotes(form.rawNotes, form.sections),
)
function addSection() {
  form.sections.push({ header: '', points: [''], hours: '' })
}
function addPoint(section: CommentSection) {
  section.points.push('')
}
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
    validateCommentSections(form.sections)
    if ([...notes.value].length > 10000)
      throw new Error('Work notes must contain at most 10,000 characters.')
    busy.value = true
    const editing = props.entry && !props.duplicate
    const input: DraftInput = {
      date: form.date,
      projectId: form.projectId,
      taskId: form.taskId,
      minutes,
      billable: form.billable,
      notes: notes.value,
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
          v-model="form.rawNotes"
          aria-label="Work notes"
          rows="4"
          maxlength="10000"
          placeholder="Optional freeform notes. These appear before formatted comments."
        />
      </label>
      <section class="comment-builder" aria-label="Formatted comments">
        <div class="comment-builder-heading">
          <div>
            <strong>Formatted comments</strong>
            <small>Break down work within this time entry.</small>
          </div>
          <button type="button" class="button secondary" @click="addSection">Add comment</button>
        </div>
        <div v-for="(section, index) in form.sections" :key="index" class="comment-section">
          <div class="comment-section-heading">
            <strong>Comment {{ index + 1 }}</strong>
            <button
              type="button"
              class="text-button"
              :aria-label="`Remove comment ${index + 1}`"
              @click="form.sections.splice(index, 1)"
            >
              Remove
            </button>
          </div>
          <label>
            Header
            <input
              v-model="section.header"
              :aria-label="`Comment ${index + 1} header`"
              placeholder="Feature or work area"
            />
          </label>
          <div class="brief-points">
            <span>Brief</span>
            <div
              v-for="(_point, pointIndex) in section.points"
              :key="pointIndex"
              class="brief-point"
            >
              <label>
                <span class="sr-only">
                  Comment {{ index + 1 }} brief point {{ pointIndex + 1 }}
                </span>
                <input
                  v-model="section.points[pointIndex]"
                  :aria-label="`Comment ${index + 1} brief point ${pointIndex + 1}`"
                  placeholder="What did you work on?"
                />
              </label>
              <button
                type="button"
                class="text-button"
                :aria-label="`Remove comment ${index + 1} brief point ${pointIndex + 1}`"
                :disabled="section.points.length === 1"
                @click="section.points.splice(pointIndex, 1)"
              >
                Remove
              </button>
            </div>
            <button type="button" class="text-button" @click="addPoint(section)">
              + Add brief point
            </button>
          </div>
          <label>
            Duration in hours
            <input
              v-model="section.hours"
              :aria-label="`Comment ${index + 1} duration in hours`"
              inputmode="decimal"
              placeholder="1.5"
            />
            <small>Update the entry duration above separately as your work grows.</small>
          </label>
        </div>
        <template v-if="form.sections.length">
          <strong class="comment-preview-label">Work notes preview</strong>
          <pre class="comment-preview">{{ notes }}</pre>
        </template>
        <small>{{ [...notes].length.toLocaleString() }} / 10,000 characters</small>
      </section>
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
