export interface Settings {
  region: string
  portalId: string
  portalName: string
  userId: string
  userName: string
  email: string
  timezone: string
  lastSync: string
}
export interface Project {
  id: string
  name: string
  active: boolean
}
export interface Task {
  id: string
  projectId: string
  name: string
  assigned: boolean
  closed: boolean
}
export interface Catalog {
  projects: Project[]
  tasks: Task[]
}
export interface Draft {
  id: string
  date: string
  projectId: string
  projectName: string
  taskId: string
  taskName: string
  minutes: number
  billable: boolean
  notes: string
  status: 'draft' | 'failed' | 'submitted' | 'uncertain' | 'submitting'
  revision: number
  accountKey: string
  remoteId: string
  error: string
  attemptAt: string
  updatedAt: string
}
export interface RemoteLog {
  id: string
  date: string
  projectId: string
  projectName: string
  taskId: string
  taskName: string
  userId: string
  minutes: number
  billable: boolean
  notes: string
  createdAt: string
  approval: string
}
export interface Total {
  key: string
  name: string
  minutes: number
}
export interface Week {
  start: string
  end: string
  entries: Draft[]
  logs: RemoteLog[]
  minutes: number
  billableMinutes: number
  days: Total[]
  projects: Total[]
}
export interface Bootstrap {
  settings: Settings
  catalog: Catalog
  token: string
  demo: boolean
  configured: boolean
  connected: boolean
  region: string
  regions: Record<string, { name: string }>
  redirectUri: string
}
export interface DraftInput {
  date: string
  projectId: string
  taskId: string
  minutes: number
  billable: boolean
  notes: string
  revision: number
}
