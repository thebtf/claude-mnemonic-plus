import { ref, watch, onUnmounted } from 'vue'
import { useSSE } from './useSSE'
import {
  fetchMaintenanceStatus,
  fetchMaintenanceLogs,
  triggerMaintenance,
  type MaintenanceStatus,
  type MaintenanceLogEntry,
} from '@/utils/api'

export function useMaintenance() {
  const status = ref<MaintenanceStatus>({
    running: false,
    current_subtask: '',
    subtasks: [],
  })
  const logs = ref<MaintenanceLogEntry[]>([])
  const autoRefresh = ref(false)
  const loading = ref(false)
  const triggering = ref(false)

  const { lastEvent } = useSSE()

  let pollTimer: ReturnType<typeof setInterval> | null = null

  async function loadStatus() {
    try {
      status.value = await fetchMaintenanceStatus()
    } catch (err) {
      console.error('[Maintenance] Failed to load status:', err)
    }
  }

  async function loadLogs() {
    try {
      const result = await fetchMaintenanceLogs(200)
      logs.value = result.entries ?? []
    } catch (err) {
      console.error('[Maintenance] Failed to load logs:', err)
    }
  }

  async function load() {
    loading.value = true
    try {
      await Promise.all([loadStatus(), loadLogs()])
    } finally {
      loading.value = false
    }
  }

  async function trigger() {
    triggering.value = true
    try {
      await triggerMaintenance()
      // Refresh status after trigger
      await loadStatus()
    } catch (err) {
      console.error('[Maintenance] Trigger failed:', err)
    } finally {
      triggering.value = false
    }
  }

  // Watch SSE events for maintenance updates
  watch(lastEvent, (event) => {
    if (!event) return
    const ev = event as any

    if (ev.type === 'maintenance_progress') {
      const { subtask, index, status: taskStatus } = ev

      // Update running state
      status.value = {
        ...status.value,
        running: true,
        current_subtask: subtask,
        subtasks: status.value.subtasks.map((st, i) => ({
          ...st,
          status: i < index - 1 ? 'completed' as const
            : i === index - 1 ? (taskStatus === 'completed' || taskStatus === 'failed' ? taskStatus : 'running') as any
            : 'pending' as const,
        })),
      }
    }

    if (ev.type === 'maintenance_complete') {
      // Refresh full status on completion
      loadStatus()
      loadLogs()
    }
  })

  // Auto-refresh polling for logs
  watch(autoRefresh, (enabled) => {
    if (enabled) {
      pollTimer = setInterval(loadLogs, 3000)
    } else if (pollTimer) {
      clearInterval(pollTimer)
      pollTimer = null
    }
  })

  onUnmounted(() => {
    if (pollTimer) {
      clearInterval(pollTimer)
      pollTimer = null
    }
  })

  return {
    status,
    logs,
    autoRefresh,
    loading,
    triggering,
    load,
    trigger,
  }
}
