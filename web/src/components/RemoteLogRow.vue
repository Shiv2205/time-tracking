<script setup lang="ts">
import { computed } from 'vue'
import type { RemoteLog } from '../types'
import { hoursLabel, projectColor, shortDate } from '../helpers'
import { readableRemoteNotes } from '../remoteNotes'
import Icon from './Icon.vue'

const props = defineProps<{ log: RemoteLog }>()
const notes = computed(() => readableRemoteNotes(props.log.notes))
</script>

<template>
  <div class="entry-row">
    <div class="entry-project-icon" :style="{ color: projectColor(log.projectId) }">
      <Icon name="link" />
    </div>
    <div class="entry-description">
      <strong>{{ log.taskName }}</strong>
      <div class="entry-meta">
        {{ log.projectName }}
        <i />
        {{ shortDate(log.date) }}
        <i />
        {{ log.billable ? 'Billable' : 'Non-billable' }}
      </div>
      <p v-if="notes" class="entry-notes">{{ notes }}</p>
      <small class="muted">Zoho ID {{ log.id }}</small>
    </div>
    <div class="entry-side">
      <strong>{{ hoursLabel(log.minutes) }}</strong>
      <span class="status-badge submitted">{{ log.approval || 'Logged' }}</span>
    </div>
  </div>
</template>
