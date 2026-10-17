<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { api, setToken } from './api'
import {
  editable,
  hoursLabel,
  projectColor,
  shiftDate,
  shortDate,
  today,
  weekday,
  weekStart,
} from './helpers'
import type { Bootstrap, Draft, Total, Week } from './types'
import Icon from './components/Icon.vue'
import EntryDialog from './components/EntryDialog.vue'
import ReviewDialog from './components/ReviewDialog.vue'
import ReportDialog from './components/ReportDialog.vue'
import ConnectionDialog from './components/ConnectionDialog.vue'
import ReconcileDialog from './components/ReconcileDialog.vue'
import RemoteLogRow from './components/RemoteLogRow.vue'

const bootstrap = ref<Bootstrap>(),
  week = ref<Week>(),
  start = ref(weekStart())
const view = ref<'drafts' | 'zoho' | 'history'>('drafts'),
  projectFilter = ref(''),
  dayFilter = ref(''),
  search = ref('')
const loading = ref(true),
  syncing = ref(false),
  error = ref(''),
  toast = ref('')
const entryOpen = ref(false),
  editing = ref<Draft>(),
  duplicating = ref(false),
  reviewOpen = ref(false),
  reportOpen = ref(false),
  connectionOpen = ref(false),
  reconciling = ref<Draft>()
const pending = computed(() => week.value?.entries.filter(editable) || [])
const matching = (entry: {
  projectId: string
  date: string
  taskName: string
  notes: string
  projectName: string
}) =>
  (!projectFilter.value || entry.projectId === projectFilter.value) &&
  (!dayFilter.value || entry.date === dayFilter.value) &&
  (!search.value ||
    `${entry.taskName} ${entry.notes} ${entry.projectName}`
      .toLowerCase()
      .includes(search.value.toLowerCase()))
const visibleDrafts = computed(() =>
  (week.value?.entries || []).filter((e) => e.status !== 'submitted' && matching(e)),
)
const visiblePending = computed(() => visibleDrafts.value.filter(editable))
const visibleHistory = computed(() =>
  (week.value?.entries || []).filter((e) => e.status === 'submitted' && matching(e)),
)
const visibleLogs = computed(() => (week.value?.logs || []).filter(matching))
const fullSubmittedMinutes = computed(() =>
  (week.value?.logs || []).reduce((sum, log) => sum + log.minutes, 0),
)
const submittedMinutes = computed(() =>
  visibleLogs.value.reduce((sum, log) => sum + log.minutes, 0),
)
const submittedBillableMinutes = computed(() =>
  visibleLogs.value.filter((log) => log.billable).reduce((sum, log) => sum + log.minutes, 0),
)
const submittedProjectCount = computed(
  () => new Set(visibleLogs.value.map((log) => log.projectId)).size,
)
const totalMinutes = computed(() => visiblePending.value.reduce((sum, e) => sum + e.minutes, 0))
const billableMinutes = computed(() =>
  visiblePending.value.filter((e) => e.billable).reduce((sum, e) => sum + e.minutes, 0),
)
const projectTotals = computed(() => {
  const totals = new Map<string, Total>()
  for (const e of pending.value) {
    const total = totals.get(e.projectId) || { key: e.projectId, name: e.projectName, minutes: 0 }
    total.minutes += e.minutes
    totals.set(e.projectId, total)
  }
  return [...totals.values()].sort((a, b) => b.minutes - a.minutes)
})
const submittedProjectTotals = computed(() => {
  const totals = new Map<string, Total>()
  for (const log of week.value?.logs || []) {
    const total = totals.get(log.projectId) || {
      key: log.projectId,
      name: log.projectName,
      minutes: 0,
    }
    total.minutes += log.minutes
    totals.set(log.projectId, total)
  }
  return [...totals.values()].sort((a, b) => b.minutes - a.minutes)
})
const days = computed(() =>
  Array.from({ length: 7 }, (_, i) => {
    const date = shiftDate(start.value, i)
    const draftMinutes = pending.value
      .filter(
        (e) => e.date === date && (!projectFilter.value || e.projectId === projectFilter.value),
      )
      .reduce((sum, e) => sum + e.minutes, 0)
    const logMinutes = (week.value?.logs || [])
      .filter(
        (log) =>
          log.date === date && (!projectFilter.value || log.projectId === projectFilter.value),
      )
      .reduce((sum, log) => sum + log.minutes, 0)
    return { date, draftMinutes, logMinutes }
  }),
)
const maxDay = computed(() =>
  Math.max(480, ...days.value.flatMap((d) => [d.draftMinutes, d.logMinutes])),
)
const ready = computed(() => bootstrap.value?.connected && !!bootstrap.value.settings.userId)
const initials = computed(() =>
  (bootstrap.value?.settings.userName || 'Your workspace')
    .split(' ')
    .slice(0, 2)
    .map((s) => s[0])
    .join(''),
)
const pageTitle = computed(() =>
  view.value === 'zoho'
    ? 'Your Zoho time logs'
    : view.value === 'history'
      ? 'Submission history'
      : 'Your week, at a glance',
)
const syncLabel = computed(() =>
  bootstrap.value?.settings.lastSync
    ? new Date(bootstrap.value.settings.lastSync).toLocaleTimeString([], {
        hour: 'numeric',
        minute: '2-digit',
      })
    : 'Not yet synced',
)
let loadNumber = 0,
  toastTimer: ReturnType<typeof setTimeout> | undefined
function notify(message: string) {
  toast.value = message
  clearTimeout(toastTimer)
  toastTimer = setTimeout(() => {
    toast.value = ''
  }, 6000)
}
async function loadWeek() {
  const request = ++loadNumber
  loading.value = true
  week.value = undefined
  try {
    const result = await api<Week>(`/week?start=${start.value}`)
    if (request === loadNumber) week.value = result
  } catch (e) {
    if (request === loadNumber) error.value = (e as Error).message
  } finally {
    if (request === loadNumber) loading.value = false
  }
}
async function loadBootstrap() {
  const result = await api<Bootstrap>('/bootstrap')
  bootstrap.value = result
  setToken(result.token)
}
async function refresh() {
  syncing.value = true
  error.value = ''
  try {
    await api('/sync', 'POST', { start: start.value })
    await loadBootstrap()
    await loadWeek()
    notify(
      bootstrap.value?.demo
        ? 'Demo reference data refreshed.'
        : 'Zoho projects, tasks, and time logs refreshed.',
    )
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    syncing.value = false
  }
}
function navigate(days: number) {
  start.value = shiftDate(start.value, days)
  dayFilter.value = ''
}
function goToToday() {
  start.value = weekStart()
  dayFilter.value = ''
}
function add(entry?: Draft, duplicate = false) {
  editing.value = entry
  duplicating.value = duplicate
  entryOpen.value = true
}
async function saved() {
  entryOpen.value = false
  await loadWeek()
  notify('Draft saved on this computer.')
}
async function remove(entry: Draft) {
  if (!window.confirm(`Delete the draft for ${entry.taskName}?`)) return
  try {
    await api(`/drafts/${entry.id}?revision=${entry.revision}`, 'DELETE')
    await loadWeek()
    notify('Draft deleted.')
  } catch (e) {
    error.value = (e as Error).message
  }
}
function report() {
  if (!projectTotals.value.length) {
    notify('Add a draft to generate a project report.')
    return
  }
  reportOpen.value = true
}
async function submitted(message: string) {
  reviewOpen.value = false
  await loadWeek()
  notify(message)
}
async function connectionChanged() {
  try {
    await loadBootstrap()
    if (ready.value) await refresh()
    else await loadWeek()
  } catch (e) {
    error.value = (e as Error).message
  }
}
function chooseProject(id: string) {
  projectFilter.value = projectFilter.value === id ? '' : id
  dayFilter.value = ''
}
watch(start, loadWeek)
onMounted(async () => {
  try {
    await loadBootstrap()
    const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone
    if (timezone && bootstrap.value?.settings.timezone !== timezone) {
      await api('/settings', 'PUT', { timezone })
      await loadBootstrap()
    }
    await loadWeek()
    const result = new URLSearchParams(window.location.search).get('connection')
    if (result) {
      history.replaceState({}, '', '/')
      if (result === 'ready') connectionOpen.value = true
      else
        error.value =
          'Zoho connection was not completed. Check your client credentials, data center, and redirect URL.'
    }
    if (bootstrap.value?.connected && !bootstrap.value.settings.portalId)
      connectionOpen.value = true
  } catch (e) {
    error.value = (e as Error).message
    loading.value = false
  }
})
</script>

<template>
  <div class="app-shell">
    <aside class="sidebar">
      <a class="brand" href="/" aria-label="Tempo home">
        <span class="brand-mark"><Icon name="clock" :size="24" /></span>
        tempo
        <span class="brand-dot">.</span>
      </a>
      <div class="workspace-name">
        <span class="workspace-symbol">W</span>
        <div>
          <strong>My workspace</strong>
          <small>Make time count</small>
        </div>
      </div>
      <p class="nav-heading">WORKSPACE</p>
      <nav aria-label="Main navigation">
        <button :class="{ active: view === 'drafts' }" @click="view = 'drafts'">
          <Icon name="grid" />
          Weekly overview
          <span v-if="pending.length" class="nav-count">{{ pending.length }}</span>
        </button>
        <button :class="{ active: view === 'zoho' }" @click="view = 'zoho'">
          <Icon name="link" />
          Zoho time logs
        </button>
        <button :class="{ active: view === 'history' }" @click="view = 'history'">
          <Icon name="check" />
          Submitted time
        </button>
        <button @click="report">
          <Icon name="report" />
          Project reports
        </button>
      </nav>
      <p class="nav-heading project-heading">
        YOUR PROJECTS
        <span>{{ bootstrap?.catalog.projects.filter((p) => p.active).length || 0 }}</span>
      </p>
      <div class="project-nav">
        <button
          v-for="p in bootstrap?.catalog.projects.filter((p) => p.active)"
          :key="p.id"
          :class="{ selected: projectFilter === p.id }"
          @click="chooseProject(p.id)"
        >
          <span class="project-dot" :style="{ background: projectColor(p.id) }" />
          <span>{{ p.name }}</span>
        </button>
        <p v-if="!bootstrap?.catalog.projects.length" class="muted small">
          Projects appear after you connect.
        </p>
      </div>
      <div class="sidebar-bottom">
        <div class="connection-card">
          <span class="status-dot" :class="{ connected: ready }" />
          <div>
            <strong>
              {{
                bootstrap?.demo ? 'Demo workspace' : ready ? 'Zoho connected' : 'Connect your Zoho'
              }}
            </strong>
            <small>
              {{
                bootstrap?.demo
                  ? 'Sample data · local only'
                  : ready
                    ? `Last refresh ${syncLabel}`
                    : 'Bring your projects together'
              }}
            </small>
          </div>
        </div>
        <button class="sidebar-settings" @click="connectionOpen = true">
          <Icon name="settings" :size="18" />
          Zoho settings
        </button>
        <a class="sidebar-settings" href="/api/backup" download>
          <Icon name="download" :size="18" />
          Back up local data
        </a>
      </div>
    </aside>

    <main>
      <header class="topbar">
        <span class="breadcrumb">
          Workspace
          <Icon name="right" :size="14" />
          <strong>
            {{
              view === 'drafts'
                ? 'Weekly overview'
                : view === 'zoho'
                  ? 'Zoho time logs'
                  : 'Submitted time'
            }}
          </strong>
        </span>
        <div class="topbar-user">
          <span v-if="bootstrap?.demo" class="demo-badge">DEMO</span>
          <span class="local-pill">
            <span />
            Saved locally
          </span>
          <button
            class="icon-button mobile-settings"
            aria-label="Zoho settings"
            @click="connectionOpen = true"
          >
            <Icon name="settings" :size="18" />
          </button>
          <span class="avatar" :title="bootstrap?.settings.userName">{{ initials }}</span>
        </div>
      </header>
      <nav class="mobile-nav" aria-label="Mobile navigation">
        <button :class="{ active: view === 'drafts' }" @click="view = 'drafts'">
          <Icon name="grid" :size="16" />
          Week
        </button>
        <button :class="{ active: view === 'zoho' }" @click="view = 'zoho'">
          <Icon name="link" :size="16" />
          Zoho logs
        </button>
        <button :class="{ active: view === 'history' }" @click="view = 'history'">
          <Icon name="check" :size="16" />
          Submitted
        </button>
        <button @click="report">
          <Icon name="report" :size="16" />
          Reports
        </button>
      </nav>
      <div class="main-content">
        <div class="page-heading">
          <div>
            <p class="eyebrow">A LITTLE CLARITY FOR YOUR WORKDAY</p>
            <h1>{{ pageTitle }}</h1>
            <p class="subtitle">Capture your work. Review your time. Wrap up with confidence.</p>
          </div>
          <button class="button primary add-time" :disabled="!ready" @click="add()">
            <Icon name="plus" :size="19" />
            Add time
          </button>
        </div>
        <div v-if="error" class="error-banner" role="alert">
          <Icon name="info" />
          <span>{{ error }}</span>
          <button class="icon-button" aria-label="Dismiss error" @click="error = ''">
            <Icon name="close" />
          </button>
        </div>
        <div v-if="bootstrap && !ready" class="connect-banner">
          <div>
            <strong>Your projects, all in one place.</strong>
            <p>Connect Zoho Projects to start preparing your time entries.</p>
          </div>
          <button class="button primary" @click="connectionOpen = true">
            Connect Zoho
            <Icon name="arrow" :size="17" />
          </button>
        </div>
        <section class="week-toolbar" aria-label="Week navigation">
          <div class="week-controls">
            <span class="calendar-icon"><Icon name="clock" :size="18" /></span>
            <strong>
              {{ shortDate(start) }} – {{ shortDate(shiftDate(start, 6)) }}, {{ start.slice(0, 4) }}
            </strong>
            <span v-if="start === weekStart()" class="this-week">This week</span>
            <div class="week-arrows">
              <button class="icon-button" aria-label="Previous week" @click="navigate(-7)">
                <Icon name="left" :size="18" />
              </button>
              <button class="icon-button" aria-label="Next week" @click="navigate(7)">
                <Icon name="right" :size="18" />
              </button>
            </div>
            <button v-if="start !== weekStart()" class="text-button" @click="goToToday">
              Today
            </button>
          </div>
          <button class="refresh-button" :disabled="syncing || !ready" @click="refresh">
            <Icon name="refresh" :size="16" :class="{ spinning: syncing }" />
            {{ syncing ? 'Refreshing…' : 'Refresh Zoho' }}
          </button>
        </section>

        <section class="stats" aria-label="Draft summary">
          <article class="stat-card main-stat">
            <div class="stat-top">
              <span>Draft time this week</span>
              <span class="stat-icon"><Icon name="clock" :size="19" /></span>
            </div>
            <div class="stat-value">
              {{ hoursLabel(totalMinutes) }}
              <span class="draft-chip">Drafts</span>
            </div>
            <p>
              {{ visiblePending.length }}
              {{ visiblePending.length === 1 ? 'entry' : 'entries' }} waiting for your review
            </p>
          </article>
          <article class="stat-card">
            <div class="stat-top">
              <span>Projects worked on</span>
              <span class="stat-icon"><Icon name="folder" :size="19" /></span>
            </div>
            <div class="stat-value">
              {{ new Set(visiblePending.map((e) => e.projectId)).size }}
              <span class="stat-unit">projects</span>
            </div>
            <p>A clear picture of where your time goes</p>
          </article>
          <article class="stat-card">
            <div class="stat-top">
              <span>Billable draft time</span>
              <span class="stat-icon"><Icon name="check" :size="19" /></span>
            </div>
            <div class="stat-value">{{ hoursLabel(billableMinutes) }}</div>
            <p>
              {{ totalMinutes ? Math.round((billableMinutes / totalMinutes) * 100) : 0 }}% of your
              drafted time
            </p>
          </article>
        </section>

        <section class="submitted-overview" aria-label="Zoho time summary">
          <div class="submitted-overview-heading">
            <h2>Submitted Zoho time</h2>
            <span>Read only · refreshed for the selected week</span>
          </div>
          <div class="stats">
            <article class="stat-card">
              <div class="stat-top">
                <span>Total logged</span>
                <Icon name="clock" :size="19" />
              </div>
              <div class="stat-value">{{ hoursLabel(submittedMinutes) }}</div>
              <p>
                {{ visibleLogs.length }} {{ visibleLogs.length === 1 ? 'log' : 'logs' }} from Zoho
              </p>
            </article>
            <article class="stat-card">
              <div class="stat-top">
                <span>Projects worked on</span>
                <Icon name="folder" :size="19" />
              </div>
              <div class="stat-value">
                {{ submittedProjectCount }}
                <span class="stat-unit">projects</span>
              </div>
              <p>Based on submitted Zoho logs</p>
            </article>
            <article class="stat-card">
              <div class="stat-top">
                <span>Billable / non-billable</span>
                <Icon name="check" :size="19" />
              </div>
              <dl class="submitted-breakdown">
                <div>
                  <dt>Billable</dt>
                  <dd>{{ hoursLabel(submittedBillableMinutes) }}</dd>
                </div>
                <div>
                  <dt>Non-billable</dt>
                  <dd>{{ hoursLabel(submittedMinutes - submittedBillableMinutes) }}</dd>
                </div>
              </dl>
            </article>
          </div>
        </section>

        <section class="week-chart panel" aria-label="Daily draft and Zoho hours">
          <div class="panel-heading">
            <h2>Your week in focus</h2>
            <span class="chart-legend">
              <span />
              Draft hours
              <span class="zoho-legend-dot" />
              Zoho hours
              <small>· Monday–Sunday</small>
            </span>
          </div>
          <div class="day-chart">
            <button
              v-for="d in days"
              :key="d.date"
              class="day-column"
              :class="{ today: d.date === today(), chosen: dayFilter === d.date }"
              :aria-label="`${weekday(d.date)} ${shortDate(d.date)}, ${hoursLabel(d.draftMinutes)} drafts and ${hoursLabel(d.logMinutes)} Zoho logs. Filter entries`"
              :aria-pressed="dayFilter === d.date"
              @click="dayFilter = dayFilter === d.date ? '' : d.date"
            >
              <span class="day-duration">
                {{
                  d.draftMinutes || d.logMinutes ? hoursLabel(d.draftMinutes + d.logMinutes) : '—'
                }}
              </span>
              <span class="bar-track">
                <span
                  class="bar"
                  :style="{
                    height: `${d.draftMinutes ? Math.max(6, (d.draftMinutes / maxDay) * 100) : 3}%`,
                  }"
                />
                <span
                  class="bar zoho-bar"
                  :style="{
                    height: `${d.logMinutes ? Math.max(6, (d.logMinutes / maxDay) * 100) : 3}%`,
                  }"
                />
              </span>
              <span class="day-name">
                {{ weekday(d.date) }}
                <i v-if="d.date === today()" />
              </span>
              <span class="day-date">{{ d.date.slice(8) }}</span>
            </button>
          </div>
        </section>

        <div class="workspace-grid">
          <section class="entries-panel panel">
            <div class="panel-heading entries-heading">
              <h2>
                {{
                  view === 'drafts'
                    ? 'Time entries'
                    : view === 'zoho'
                      ? 'Existing Zoho logs'
                      : 'Submitted entries'
                }}
                <span class="count-badge">
                  {{
                    view === 'drafts'
                      ? visibleDrafts.length
                      : view === 'zoho'
                        ? visibleLogs.length
                        : visibleHistory.length
                  }}
                </span>
              </h2>
              <span class="subtle-label">
                {{
                  view === 'drafts'
                    ? 'Local drafts'
                    : view === 'zoho'
                      ? 'Read only · reference'
                      : 'Confirmed submissions'
                }}
              </span>
            </div>
            <div class="entry-filters">
              <label class="search-field">
                <Icon name="folder" :size="17" />
                <input
                  v-model="search"
                  type="search"
                  aria-label="Search entries"
                  placeholder="Search entries…"
                />
              </label>
              <select v-model="projectFilter" aria-label="Filter by project">
                <option value="">All projects</option>
                <option v-for="p in bootstrap?.catalog.projects" :key="p.id" :value="p.id">
                  {{ p.name }}
                </option>
              </select>
              <button v-if="dayFilter" class="filter-chip" @click="dayFilter = ''">
                {{ shortDate(dayFilter) }}
                <Icon name="close" :size="13" />
              </button>
            </div>
            <div v-if="loading" class="empty-state">
              <div class="loading-dot" />
              Loading your week…
            </div>
            <template v-else-if="view === 'drafts'">
              <div v-if="!visibleDrafts.length" class="empty-state">
                <span class="empty-icon"><Icon name="clock" :size="28" /></span>
                <h3>A fresh page for your week</h3>
                <p>
                  {{
                    ready
                      ? 'Add a time entry to keep track of what you’re working on.'
                      : 'Connect your Zoho account to see your projects and tasks.'
                  }}
                </p>
                <button class="button secondary" :disabled="!ready" @click="add()">
                  <Icon name="plus" :size="17" />
                  Add your first entry
                </button>
              </div>
              <div
                v-for="entry in visibleDrafts"
                :key="entry.id"
                class="entry-row"
                :data-testid="`entry-${entry.id}`"
              >
                <div
                  class="entry-project-icon"
                  :style="{
                    color: projectColor(entry.projectId),
                    background: `${projectColor(entry.projectId)}14`,
                  }"
                >
                  <Icon name="folder" :size="20" />
                </div>
                <div class="entry-description">
                  <div class="entry-title">
                    <strong>{{ entry.taskName }}</strong>
                    <span
                      v-if="entry.status === 'failed' || entry.status === 'uncertain'"
                      class="status-badge"
                      :class="entry.status"
                    >
                      {{ entry.status === 'failed' ? 'Needs attention' : 'Check submission' }}
                    </span>
                  </div>
                  <div class="entry-meta">
                    <span>{{ entry.projectName }}</span>
                    <i />
                    {{ shortDate(entry.date) }}
                    <i />
                    <span :class="{ billable: entry.billable }">
                      {{ entry.billable ? 'Billable' : 'Non-billable' }}
                    </span>
                  </div>
                  <p v-if="entry.notes" class="entry-notes">{{ entry.notes }}</p>
                  <p v-if="entry.error" class="entry-error">{{ entry.error }}</p>
                </div>
                <div class="entry-side">
                  <strong>{{ hoursLabel(entry.minutes) }}</strong>
                  <span v-if="editable(entry)" class="status-badge draft">Draft</span>
                </div>
                <div class="entry-actions">
                  <template v-if="editable(entry)">
                    <button
                      class="icon-button"
                      :aria-label="`Edit ${entry.taskName}`"
                      @click="add(entry)"
                    >
                      <Icon name="edit" :size="16" />
                    </button>
                    <button
                      class="icon-button"
                      :aria-label="`Duplicate ${entry.taskName}`"
                      @click="add(entry, true)"
                    >
                      <Icon name="copy" :size="16" />
                    </button>
                    <button
                      class="icon-button delete-button"
                      :aria-label="`Delete ${entry.taskName}`"
                      @click="remove(entry)"
                    >
                      <Icon name="trash" :size="16" />
                    </button>
                  </template>
                  <button v-else class="text-button" @click="reconciling = entry">Check</button>
                </div>
              </div>
              <button class="inline-add" :disabled="!ready" @click="add()">
                <Icon name="plus" :size="16" />
                Add another time entry
              </button>
            </template>
            <template v-else-if="view === 'zoho'">
              <div v-if="!visibleLogs.length" class="empty-state">
                <Icon name="link" :size="28" />
                <h3>No reference logs for this week</h3>
                <p>Refresh Zoho to fetch your existing task time logs.</p>
              </div>
              <RemoteLogRow v-for="log in visibleLogs" :key="log.id" :log="log" />
            </template>
            <template v-else>
              <div v-if="!visibleHistory.length" class="empty-state">
                <Icon name="check" :size="28" />
                <h3>Your submitted work will appear here</h3>
                <p>Review and submit drafts to keep a record of each confirmed result.</p>
              </div>
              <div v-for="entry in visibleHistory" :key="entry.id" class="entry-row">
                <div class="entry-project-icon"><Icon name="check" /></div>
                <div class="entry-description">
                  <strong>{{ entry.taskName }}</strong>
                  <div class="entry-meta">
                    {{ entry.projectName }}
                    <i />
                    {{ shortDate(entry.date) }}
                  </div>
                  <p class="entry-notes">{{ entry.notes }}</p>
                  <small class="muted">
                    {{ bootstrap?.demo ? 'Simulated ID' : 'Zoho ID' }} {{ entry.remoteId }}
                  </small>
                </div>
                <div class="entry-side">
                  <strong>{{ hoursLabel(entry.minutes) }}</strong>
                  <span class="status-badge submitted">
                    {{ bootstrap?.demo ? 'Simulated' : 'Submitted' }}
                  </span>
                </div>
              </div>
            </template>
            <footer class="entries-footer">
              <span>
                <Icon name="info" :size="14" />
                {{
                  view === 'drafts'
                    ? 'Your drafts stay here until you submit them.'
                    : 'These entries are excluded from draft totals.'
                }}
              </span>
              <strong v-if="view === 'drafts'">{{ hoursLabel(totalMinutes) }} total</strong>
            </footer>
          </section>

          <aside class="week-aside">
            <section class="project-summary panel">
              <div class="panel-heading">
                <h2>Time by project</h2>
                <Icon name="folder" :size="17" />
              </div>
              <p class="muted small">Drafts across the full week</p>
              <div v-if="!projectTotals.length" class="small muted project-summary-empty">
                Your project breakdown will appear here.
              </div>
              <button
                v-for="p in projectTotals"
                :key="p.key"
                class="project-total"
                @click="chooseProject(p.key)"
              >
                <div>
                  <span class="project-dot" :style="{ background: projectColor(p.key) }" />
                  <strong>{{ p.name }}</strong>
                  <span>{{ hoursLabel(p.minutes) }}</span>
                </div>
                <span class="project-progress">
                  <span
                    :style="{
                      width: `${(p.minutes / (week?.minutes || 1)) * 100}%`,
                      background: projectColor(p.key),
                    }"
                  />
                </span>
              </button>
              <div class="project-summary-total">
                <span>Weekly total</span>
                <strong>{{ hoursLabel(week?.minutes || 0) }}</strong>
              </div>
            </section>
            <section class="project-summary panel" aria-label="Zoho time by project">
              <div class="panel-heading">
                <h2>Zoho time by project</h2>
                <Icon name="link" :size="17" />
              </div>
              <p class="muted small">Submitted logs across the full week</p>
              <div v-if="!submittedProjectTotals.length" class="small muted project-summary-empty">
                Refresh Zoho for this week to see submitted projects.
              </div>
              <button
                v-for="p in submittedProjectTotals"
                :key="p.key"
                class="project-total"
                @click="chooseProject(p.key)"
              >
                <div>
                  <span class="project-dot" :style="{ background: projectColor(p.key) }" />
                  <strong>{{ p.name }}</strong>
                  <span>{{ hoursLabel(p.minutes) }}</span>
                </div>
                <span class="project-progress">
                  <span
                    :style="{
                      width: `${(p.minutes / (fullSubmittedMinutes || 1)) * 100}%`,
                      background: projectColor(p.key),
                    }"
                  />
                </span>
              </button>
              <div class="project-summary-total">
                <span>Submitted total</span>
                <strong>{{ hoursLabel(fullSubmittedMinutes) }}</strong>
              </div>
            </section>
            <section class="report-card">
              <span class="report-symbol"><Icon name="report" :size="22" /></span>
              <h2>
                A good week deserves
                <br />
                a clear report.
              </h2>
              <p>Turn your project entries and work notes into a ready-to-share update.</p>
              <button class="button report-button" @click="report">
                Generate report
                <Icon name="arrow" :size="16" />
              </button>
              <small>Made from your drafts · ready to copy</small>
            </section>
          </aside>
        </div>
        <section
          v-if="view === 'drafts'"
          class="dashboard-logs panel"
          aria-label="Submitted Zoho logs"
        >
          <div class="panel-heading entries-heading">
            <h2>
              Submitted Zoho logs
              <span class="count-badge">{{ visibleLogs.length }}</span>
            </h2>
            <span class="subtle-label">Read only · from Zoho</span>
          </div>
          <div v-if="!visibleLogs.length && !loading" class="empty-state">
            <Icon name="link" :size="28" />
            <h3>No Zoho logs for this week</h3>
            <p>Use Refresh Zoho to load the selected week.</p>
          </div>
          <RemoteLogRow v-for="log in visibleLogs" :key="log.id" :log="log" />
        </section>
        <section v-if="view === 'drafts'" class="review-banner">
          <span class="review-check"><Icon name="check" :size="21" /></span>
          <div>
            <h3>Ready to wrap up your week?</h3>
            <p>Take a moment to review your drafts before sending them to Zoho.</p>
          </div>
          <button
            class="button primary"
            :disabled="!visiblePending.length || !ready"
            @click="reviewOpen = true"
          >
            Review {{ visiblePending.length }}
            {{ visiblePending.length === 1 ? 'entry' : 'entries' }}
            <Icon name="arrow" :size="17" />
          </button>
        </section>
        <footer class="page-footer">
          <span>More focus on your work. Less time on your timesheet.</span>
          <span>Local timezone: {{ bootstrap?.settings.timezone || '…' }}</span>
        </footer>
      </div>
    </main>
    <div v-if="toast" class="toast" role="status">
      <Icon name="check" :size="18" />
      {{ toast }}
      <button class="icon-button" aria-label="Dismiss notification" @click="toast = ''">
        <Icon name="close" :size="15" />
      </button>
    </div>
    <EntryDialog
      v-if="entryOpen && bootstrap"
      :catalog="bootstrap.catalog"
      :entry="editing"
      :duplicate="duplicating"
      :initial-date="dayFilter || (start === weekStart() ? today() : start)"
      @close="entryOpen = false"
      @saved="saved"
    />
    <ReviewDialog
      v-if="reviewOpen"
      :entries="visiblePending"
      :demo="bootstrap?.demo || false"
      @close="reviewOpen = false"
      @submitted="submitted"
    />
    <ReportDialog
      v-if="reportOpen"
      :projects="projectTotals"
      :start="start"
      :initial-project="
        projectTotals.some((p) => p.key === projectFilter) ? projectFilter : undefined
      "
      @close="reportOpen = false"
    />
    <ConnectionDialog
      v-if="connectionOpen && bootstrap"
      :bootstrap="bootstrap"
      @close="connectionOpen = false"
      @changed="connectionChanged"
    />
    <ReconcileDialog
      v-if="reconciling"
      :entry="reconciling"
      @close="reconciling = undefined"
      @changed="loadWeek"
    />
  </div>
</template>
