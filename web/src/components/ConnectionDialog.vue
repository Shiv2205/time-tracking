<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api } from '../api'
import type { Bootstrap } from '../types'
import Modal from './Modal.vue'
import Icon from './Icon.vue'
const props = defineProps<{ bootstrap: Bootstrap }>()
const emit = defineEmits<{ close: []; changed: [] }>()
const region = ref(props.bootstrap.region || 'us'),
  clientId = ref(''),
  clientSecret = ref(''),
  busy = ref(false),
  error = ref('')
const portals = ref<{ id: string; name: string }[]>([]),
  portalId = ref(props.bootstrap.settings.portalId),
  configured = ref(props.bootstrap.configured)
async function configure() {
  busy.value = true
  error.value = ''
  try {
    await api('/connection/config', 'POST', {
      region: region.value,
      clientId: clientId.value,
      clientSecret: clientSecret.value,
    })
    clientSecret.value = ''
    configured.value = true
    emit('changed')
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    busy.value = false
  }
}
async function connect() {
  busy.value = true
  error.value = ''
  try {
    const result = await api<{ url: string }>('/connection/auth', 'POST', {})
    window.location.assign(result.url)
  } catch (e) {
    error.value = (e as Error).message
    busy.value = false
  }
}
async function selectPortal() {
  busy.value = true
  error.value = ''
  try {
    await api('/connection/portal', 'POST', { portalId: portalId.value })
    emit('changed')
    emit('close')
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    busy.value = false
  }
}
async function disconnect() {
  if (!window.confirm('Disconnect Zoho? Your local drafts will be kept.')) return
  busy.value = true
  try {
    await api('/connection', 'DELETE')
    emit('changed')
    emit('close')
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    busy.value = false
  }
}
onMounted(async () => {
  if (!props.bootstrap.connected || props.bootstrap.demo) return
  busy.value = true
  try {
    portals.value = await api('/connection/portals')
    if (!portalId.value) portalId.value = portals.value[0]?.id || ''
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    busy.value = false
  }
})
</script>
<template>
  <Modal title="Zoho connection" :busy="busy" @close="!busy && emit('close')">
    <div class="modal-body">
      <div v-if="bootstrap.demo" class="notice">
        <Icon name="info" />
        <span>
          You’re in demo mode. Time entries and submissions use separate sample data. Start the app
          without
          <code>-demo</code>
          to connect your real account.
        </span>
      </div>
      <template v-else-if="bootstrap.connected">
        <p class="muted">
          Choose the portal containing your work projects. Your Zoho account determines the user
          whose time is shown.
        </p>
        <p v-if="bootstrap.settings.email" class="account-detail">
          {{ bootstrap.settings.userName }}
          <small>{{ bootstrap.settings.email }}</small>
        </p>
        <form @submit.prevent="selectPortal">
          <label>
            Zoho portal
            <select v-model="portalId" required>
              <option v-for="p in portals" :key="p.id" :value="p.id">{{ p.name }}</option>
            </select>
          </label>
          <div class="modal-footer">
            <button type="button" class="button danger-text" :disabled="busy" @click="disconnect">
              Disconnect
            </button>
            <button class="button primary" :disabled="busy || !portalId">
              {{ busy ? 'Loading…' : 'Use this portal' }}
            </button>
          </div>
        </form>
      </template>
      <template v-else>
        <p class="muted">
          Create a server-based OAuth client in your
          <a href="https://api-console.zoho.com/" target="_blank" rel="noopener noreferrer">
            Zoho API console
          </a>
          , then connect your account.
        </p>
        <label>
          Authorized redirect URI
          <input :value="bootstrap.redirectUri" readonly />
          <small>Register this exact URL in your OAuth client.</small>
        </label>
        <form v-if="!configured" @submit.prevent="configure" class="entry-form">
          <label>
            Zoho data center
            <select v-model="region">
              <option v-for="(r, key) in bootstrap.regions" :key="key" :value="key">
                {{ r.name }}
              </option>
            </select>
          </label>
          <label>
            Client ID
            <input v-model="clientId" required autocomplete="off" />
          </label>
          <label>
            Client secret
            <input v-model="clientSecret" type="password" required autocomplete="off" />
            <small>Stored on this computer; never returned to the interface.</small>
          </label>
          <button class="button primary" :disabled="busy">
            {{ busy ? 'Saving…' : 'Save credentials' }}
          </button>
        </form>
        <div v-else class="configured">
          <div class="notice">
            <Icon name="check" />
            <span>OAuth client configured. Continue to Zoho to grant access.</span>
          </div>
          <button class="button primary" :disabled="busy" @click="connect">
            {{ busy ? 'Connecting…' : 'Connect to Zoho' }}
            <Icon name="arrow" :size="17" />
          </button>
          <button class="text-button" @click="configured = false">Change client credentials</button>
        </div>
      </template>
      <p v-if="error" class="error-message" role="alert">{{ error }}</p>
    </div>
  </Modal>
</template>
