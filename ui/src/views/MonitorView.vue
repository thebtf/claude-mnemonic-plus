<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted } from 'vue'
import { useLogs, type LogLevel } from '@/composables/useLogs'
import { useMaintenance } from '@/composables/useMaintenance'

// Existing logs composable
const {
  filteredEntries,
  connected,
  paused,
  enabledLevels,
  searchText,
  connect,
  togglePause,
  toggleLevel,
  clearEntries,
  LOG_LEVELS,
} = useLogs()

// Maintenance composable
const {
  status: maintStatus,
  logs: maintLogs,
  autoRefresh,
  triggering,
  load: loadMaintenance,
  trigger: triggerRun,
} = useMaintenance()

// Active tab
const activeTab = ref<'logs' | 'maintenance'>('maintenance')

const logContainer = ref<HTMLElement | null>(null)
const autoScroll = ref(true)

const LEVEL_COLORS: Record<LogLevel, { text: string; bg: string; border: string }> = {
  trace: { text: 'text-slate-500', bg: 'bg-slate-500/10', border: 'border-slate-500/30' },
  debug: { text: 'text-blue-400', bg: 'bg-blue-500/10', border: 'border-blue-500/30' },
  info: { text: 'text-green-400', bg: 'bg-green-500/10', border: 'border-green-500/30' },
  warn: { text: 'text-yellow-400', bg: 'bg-yellow-500/10', border: 'border-yellow-500/30' },
  error: { text: 'text-red-400', bg: 'bg-red-500/10', border: 'border-red-500/30' },
  fatal: { text: 'text-red-300', bg: 'bg-red-500/20', border: 'border-red-500/50' },
}

const progressPercent = computed(() => {
  if (!maintStatus.value.running || !maintStatus.value.subtasks.length) return 0
  const completed = maintStatus.value.subtasks.filter(s => s.status === 'completed').length
  return Math.round((completed / maintStatus.value.subtasks.length) * 100)
})

const maintSearchText = ref('')

const filteredMaintLogs = computed(() => {
  if (!maintSearchText.value) return maintLogs.value
  const q = maintSearchText.value.toLowerCase()
  return maintLogs.value.filter(e => e.message.toLowerCase().includes(q))
})

function scrollToBottom() {
  if (logContainer.value) {
    logContainer.value.scrollTop = logContainer.value.scrollHeight
  }
}

function handleScroll() {
  if (!logContainer.value) return
  const { scrollTop, scrollHeight, clientHeight } = logContainer.value
  autoScroll.value = scrollHeight - scrollTop - clientHeight < 50
}

watch(
  () => filteredEntries.value.length,
  () => {
    if (autoScroll.value && activeTab.value === 'logs') {
      nextTick(scrollToBottom)
    }
  }
)

function formatTimestamp(ts: string | number): string {
  try {
    const d = typeof ts === 'number' ? new Date(ts * 1000) : new Date(ts)
    return d.toLocaleTimeString('en-US', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' })
      + '.' + String(d.getMilliseconds()).padStart(3, '0')
  } catch {
    return String(ts)
  }
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  return `${(ms / 60000).toFixed(1)}m`
}

function formatRelativeTime(dateStr: string): string {
  try {
    const d = new Date(dateStr)
    const now = new Date()
    const diffMs = now.getTime() - d.getTime()
    if (diffMs < 60000) return 'just now'
    if (diffMs < 3600000) return `${Math.floor(diffMs / 60000)}m ago`
    if (diffMs < 86400000) return `${Math.floor(diffMs / 3600000)}h ago`
    return d.toLocaleDateString()
  } catch {
    return dateStr
  }
}

function subtaskSymbol(status: string): string {
  switch (status) {
    case 'completed': return '\u2713'
    case 'running': return '\u23F3'
    case 'failed': return '\u2717'
    default: return '\u25CB'
  }
}

onMounted(() => {
  connect()
  loadMaintenance()
})
</script>

<template>
  <div class="flex flex-col h-full">
    <!-- Header -->
    <div class="flex items-center justify-between mb-4">
      <div class="flex items-center gap-3">
        <i class="fas fa-display text-claude-400 text-xl" />
        <h1 class="text-2xl font-bold text-white">Monitor</h1>
      </div>

      <!-- Tab switcher -->
      <div class="flex items-center gap-1 bg-slate-800/50 rounded-lg p-1 border border-slate-700/50">
        <button
          @click="activeTab = 'maintenance'"
          :class="[
            'px-3 py-1.5 rounded-md text-sm font-medium transition-colors',
            activeTab === 'maintenance'
              ? 'bg-claude-500/20 text-claude-300'
              : 'text-slate-400 hover:text-white'
          ]"
        >
          <i class="fas fa-wrench mr-1.5" />
          Maintenance
        </button>
        <button
          @click="activeTab = 'logs'"
          :class="[
            'px-3 py-1.5 rounded-md text-sm font-medium transition-colors',
            activeTab === 'logs'
              ? 'bg-claude-500/20 text-claude-300'
              : 'text-slate-400 hover:text-white'
          ]"
        >
          <i class="fas fa-terminal mr-1.5" />
          Server Logs
        </button>
      </div>
    </div>

    <!-- Maintenance Tab -->
    <template v-if="activeTab === 'maintenance'">
      <!-- Status Panel -->
      <div class="bg-slate-800/50 rounded-xl border border-slate-700/50 p-4 mb-4">
        <div class="flex items-center justify-between mb-3">
          <div class="flex items-center gap-3">
            <span :class="[
              'w-2.5 h-2.5 rounded-full',
              maintStatus.running ? 'bg-amber-400 animate-pulse' : 'bg-green-400'
            ]" />
            <span class="text-white font-medium">
              {{ maintStatus.running ? 'Running' : 'Idle' }}
            </span>
            <span v-if="maintStatus.running && maintStatus.current_subtask" class="text-slate-400 text-sm">
              — {{ maintStatus.current_subtask }}
            </span>
          </div>

          <button
            @click="triggerRun"
            :disabled="maintStatus.running || triggering"
            :class="[
              'px-4 py-1.5 rounded-lg text-sm font-medium border transition-colors',
              maintStatus.running || triggering
                ? 'bg-slate-700/50 text-slate-500 border-slate-700/50 cursor-not-allowed'
                : 'bg-claude-500/20 text-claude-300 border-claude-500/30 hover:bg-claude-500/30'
            ]"
          >
            <i :class="['fas mr-1.5', triggering ? 'fa-spinner fa-spin' : 'fa-play']" />
            Run Now
          </button>
        </div>

        <!-- Progress bar -->
        <div v-if="maintStatus.running" class="mb-3">
          <div class="flex justify-between text-xs text-slate-400 mb-1">
            <span>Progress</span>
            <span>{{ progressPercent }}%</span>
          </div>
          <div class="w-full bg-slate-700/50 rounded-full h-2">
            <div
              class="bg-blue-500 h-2 rounded-full transition-all duration-500"
              :style="{ width: `${progressPercent}%` }"
            />
          </div>
        </div>

        <!-- Last run info (when idle) -->
        <div v-if="!maintStatus.running && maintStatus.last_run" class="text-sm text-slate-400">
          Last run: {{ formatRelativeTime(maintStatus.last_run.started_at) }}
          · Duration: {{ formatDuration(maintStatus.last_run.duration_ms) }}
          · {{ maintStatus.last_run.subtask_count }} subtasks
          <span v-if="maintStatus.next_run_at">
            · Next: {{ formatRelativeTime(maintStatus.next_run_at) }}
          </span>
        </div>

        <div v-if="!maintStatus.running && !maintStatus.last_run" class="text-sm text-slate-500">
          Never run — click "Run Now" to start
        </div>

        <!-- Subtask list -->
        <div v-if="maintStatus.running && maintStatus.subtasks.length" class="mt-3 grid grid-cols-5 gap-1">
          <div
            v-for="subtask in maintStatus.subtasks"
            :key="subtask.name"
            :title="subtask.name + ' — ' + subtask.status"
            :class="[
              'px-1.5 py-0.5 rounded text-[10px] font-mono truncate border',
              subtask.status === 'completed' ? 'bg-green-500/10 text-green-400 border-green-500/20' :
              subtask.status === 'running' ? 'bg-amber-500/10 text-amber-400 border-amber-500/20' :
              subtask.status === 'failed' ? 'bg-red-500/10 text-red-400 border-red-500/20' :
              'bg-slate-800/30 text-slate-600 border-slate-700/30'
            ]"
          >
            <span class="mr-1">{{ subtaskSymbol(subtask.status) }}</span>
            {{ subtask.name.replace(/_/g, ' ') }}
          </div>
        </div>
      </div>

      <!-- Maintenance Logs -->
      <div class="flex items-center justify-between mb-2">
        <div class="flex items-center gap-3">
          <h2 class="text-sm font-medium text-slate-300">Maintenance Logs</h2>
          <button
            @click="autoRefresh = !autoRefresh"
            :class="[
              'px-2 py-0.5 rounded text-[10px] font-medium border transition-colors',
              autoRefresh
                ? 'bg-green-500/10 text-green-400 border-green-500/30'
                : 'bg-slate-800/50 text-slate-500 border-slate-700/50 hover:text-slate-300'
            ]"
          >
            {{ autoRefresh ? 'Auto Refresh ON' : 'Auto Refresh OFF' }}
          </button>
        </div>

        <div class="relative max-w-xs">
          <i class="fas fa-search absolute left-2.5 top-1/2 -translate-y-1/2 text-slate-600 text-xs" />
          <input
            v-model="maintSearchText"
            type="text"
            placeholder="Filter maintenance logs..."
            class="w-full pl-8 pr-3 py-1.5 rounded-lg bg-slate-800/50 border border-slate-700/50 text-xs text-slate-200 placeholder-slate-600 focus:outline-none focus:ring-1 focus:ring-claude-500/50"
          />
        </div>
      </div>

      <div class="flex-1 overflow-y-auto rounded-xl border-2 border-slate-700/50 bg-slate-950/80 font-mono text-xs p-3 min-h-[300px]">
        <div v-if="filteredMaintLogs.length === 0" class="flex items-center justify-center h-full text-slate-600">
          No maintenance log entries
        </div>
        <div
          v-for="(entry, idx) in filteredMaintLogs"
          :key="idx"
          class="flex items-start gap-2 py-0.5 hover:bg-slate-800/30 px-1 rounded"
        >
          <span class="text-slate-600 whitespace-nowrap flex-shrink-0">{{ formatTimestamp(entry.timestamp) }}</span>
          <span :class="[
            'px-1.5 py-0 rounded text-[10px] font-bold uppercase flex-shrink-0 min-w-[3rem] text-center',
            entry.level === 'error' ? 'bg-red-500/10 text-red-400' :
            entry.level === 'warn' ? 'bg-yellow-500/10 text-yellow-400' :
            entry.level === 'info' ? 'bg-green-500/10 text-green-400' :
            'bg-slate-500/10 text-slate-400'
          ]">{{ entry.level }}</span>
          <span class="text-slate-300 break-all">{{ entry.message }}</span>
        </div>
      </div>
    </template>

    <!-- Server Logs Tab (existing LogsView content) -->
    <template v-if="activeTab === 'logs'">
      <div class="flex items-center justify-between mb-4">
        <div class="flex items-center gap-3">
          <span :class="[
            'flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] font-medium border',
            connected
              ? 'bg-green-500/10 text-green-400 border-green-500/30'
              : 'bg-red-500/10 text-red-400 border-red-500/30'
          ]">
            <span :class="['w-1.5 h-1.5 rounded-full', connected ? 'bg-green-400' : 'bg-red-400']" />
            {{ connected ? 'Connected' : 'Disconnected' }}
          </span>
        </div>

        <div class="flex items-center gap-2">
          <button
            @click="togglePause()"
            :class="[
              'px-3 py-1.5 rounded-lg text-sm border transition-colors',
              paused
                ? 'bg-amber-500/20 text-amber-300 border-amber-500/30'
                : 'bg-slate-800/50 text-slate-300 border-slate-700/50 hover:text-white hover:border-claude-500/50'
            ]"
          >
            <i :class="['fas mr-1.5', paused ? 'fa-play' : 'fa-pause']" />
            {{ paused ? 'Resume' : 'Pause' }}
          </button>
          <button
            @click="clearEntries()"
            class="px-3 py-1.5 rounded-lg text-sm bg-slate-800/50 border border-slate-700/50 text-slate-400 hover:text-white hover:border-slate-600 transition-colors"
          >
            <i class="fas fa-eraser mr-1.5" />
            Clear
          </button>
        </div>
      </div>

      <div class="flex items-center gap-3 mb-4">
        <div class="flex items-center gap-1">
          <button
            v-for="level in LOG_LEVELS"
            :key="level"
            @click="toggleLevel(level)"
            :class="[
              'px-2 py-1 rounded text-[10px] font-medium uppercase border transition-colors',
              enabledLevels.has(level)
                ? `${LEVEL_COLORS[level].bg} ${LEVEL_COLORS[level].text} ${LEVEL_COLORS[level].border}`
                : 'bg-slate-800/20 text-slate-600 border-slate-700/30 hover:text-slate-400'
            ]"
          >
            {{ level }}
          </button>
        </div>

        <div class="relative flex-1 max-w-xs">
          <i class="fas fa-search absolute left-2.5 top-1/2 -translate-y-1/2 text-slate-600 text-xs" />
          <input
            v-model="searchText"
            type="text"
            placeholder="Filter logs..."
            class="w-full pl-8 pr-3 py-1.5 rounded-lg bg-slate-800/50 border border-slate-700/50 text-xs text-slate-200 placeholder-slate-600 focus:outline-none focus:ring-1 focus:ring-claude-500/50"
          />
        </div>

        <span class="text-xs text-slate-600">{{ filteredEntries.length }} entries</span>
      </div>

      <div
        ref="logContainer"
        @scroll="handleScroll"
        class="flex-1 overflow-y-auto rounded-xl border-2 border-slate-700/50 bg-slate-950/80 font-mono text-xs p-3 min-h-[400px]"
      >
        <div v-if="filteredEntries.length === 0" class="flex items-center justify-center h-full text-slate-600">
          <span v-if="connected">Waiting for log entries...</span>
          <span v-else>Not connected to log stream</span>
        </div>
        <div
          v-for="entry in filteredEntries"
          :key="entry.id"
          class="flex items-start gap-2 py-0.5 hover:bg-slate-800/30 px-1 rounded"
        >
          <span class="text-slate-600 whitespace-nowrap flex-shrink-0">{{ formatTimestamp(entry.timestamp) }}</span>
          <span :class="[
            'px-1.5 py-0 rounded text-[10px] font-bold uppercase flex-shrink-0 min-w-[3rem] text-center',
            LEVEL_COLORS[entry.level]?.bg || 'bg-slate-500/10',
            LEVEL_COLORS[entry.level]?.text || 'text-slate-400',
          ]">{{ entry.level }}</span>
          <span class="text-slate-300 break-all">{{ entry.message }}</span>
        </div>
      </div>
    </template>
  </div>
</template>
