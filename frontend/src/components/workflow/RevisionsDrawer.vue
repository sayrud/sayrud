<script setup lang="ts">
import { Message, Modal } from '@arco-design/web-vue'
import { Info, RotateCcw } from '@lucide/vue'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { workflowsApi } from '@/api/workflow'
import StateIllustration from '@/components/common/StateIllustration.vue'
import UserAvatar from '@/components/common/UserAvatar.vue'
import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import type { TriggerType, WorkflowRevision } from '@/types/workflow'
import { METHOD_COLORS, TRIGGER_META } from '@/utils/workflow'

dayjs.extend(relativeTime)

const visible = defineModel<boolean>('visible', { required: true })
const { t, locale } = useI18n()
const store = useWorkflowEditorStore()

const revisions = ref<WorkflowRevision[]>([])
const loading = ref(false)
const loaded = ref(false)

const currentVersion = computed(() => store.workflow?.publishedVersion ?? 0)
const current = computed(() => revisions.value.find((r) => r.version === currentVersion.value))

// The earlier versions are grouped by publish date; the current version is shown in the card above.
const groups = computed(() => {
  const out: { key: string; label: string; items: WorkflowRevision[] }[] = []
  for (const r of revisions.value) {
    if (r.version === currentVersion.value) continue
    const key = dayjs(r.createdAt).format('YYYY-MM-DD')
    let group = out[out.length - 1]
    if (group?.key !== key) {
      group = { key, label: dayLabel(r.createdAt), items: [] }
      out.push(group)
    }
    group.items.push(r)
  }
  return out
})

function dayLabel(v: string) {
  const d = dayjs(v)
  if (d.isSame(dayjs(), 'day')) return t('workflow.revisions.today')
  if (d.isSame(dayjs().subtract(1, 'day'), 'day')) return t('workflow.revisions.yesterday')
  const sameYear = d.isSame(dayjs(), 'year')
  return d.toDate().toLocaleDateString(locale.value, { year: sameYear ? undefined : 'numeric', month: 'long', day: 'numeric' })
}

const triggerKey = (r?: WorkflowRevision) => (r ? `${r.triggerType} ${r.method} ${r.path}` : '')
const triggerMeta = (r: WorkflowRevision) => TRIGGER_META[r.triggerType as TriggerType]
const fullTime = (v: string) => dayjs(v).format('YYYY-MM-DD HH:mm:ss')

async function load() {
  if (!store.workflow) return
  loading.value = true
  try {
    revisions.value = await workflowsApi.revisions(store.projectUID, store.workflow.uid)
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    loading.value = false
    loaded.value = true
  }
}

watch(visible, (v) => {
  if (v) void load()
})

function rollback(r: WorkflowRevision) {
  const wf = store.workflow
  if (!wf) return
  Modal.warning({
    title: t('workflow.revisions.rollbackTitle', { version: r.version }),
    content: t(wf.hasUnpublishedChanges ? 'workflow.revisions.rollbackContentDirty' : 'workflow.revisions.rollbackContent', {
      version: r.version,
      next: currentVersion.value + 1,
    }),
    hideCancel: false,
    okText: t('workflow.revisions.rollback'),
    onBeforeOk: async () => {
      try {
        // Save pending edits first so that draftVersion is up to date.
        await store.flush()
        const resp = await workflowsApi.rollback(store.projectUID, wf.uid, r.version, store.draftVersion)
        if (resp.workflow) store.setWorkflow(resp.workflow)
        Message.success(t('workflow.revisions.rolledBack', { version: r.version, next: currentVersion.value }))
        void load()
        return true
      } catch (e) {
        Message.error(e instanceof Error ? e.message : String(e))
        return false
      }
    },
  })
}
</script>

<template>
  <a-drawer v-model:visible="visible" :width="480" :footer="false" unmount-on-close class="revisions-drawer">
    <template #title>
      <span class="drawer-title">
        {{ t('workflow.revisions.title') }}
        <span v-if="revisions.length" class="count">{{ revisions.length }}</span>
      </span>
    </template>

    <div v-if="!loaded" class="skeleton">
      <a-skeleton animation>
        <a-skeleton-shape class="skeleton-card" />
        <a-skeleton-line :rows="5" :line-height="20" :line-spacing="16" />
      </a-skeleton>
    </div>

    <div v-else-if="!revisions.length" class="empty">
      <StateIllustration name="empty-folder" :width="128" />
      <div class="empty-title">{{ t('workflow.revisions.empty') }}</div>
      <div class="empty-text">{{ t('workflow.revisions.emptyHint') }}</div>
    </div>

    <template v-else>
      <section v-if="current" class="current">
        <div class="current-head">
          <span class="eyebrow">{{ t('workflow.revisions.current') }}</span>
          <a-tag size="small" :color="store.workflow?.enabled ? 'green' : 'gray'" class="status">
            <span class="status-dot" />
            {{ store.workflow?.enabled ? t('workflow.revisions.live') : t('workflow.revisions.disabled') }}
          </a-tag>
        </div>
        <div class="current-version">
          {{ t('workflow.editor.version', { version: current.version }) }}
          <a-tag v-if="current.restoredFrom" size="small" class="restored">
            <template #icon><RotateCcw :size="11" /></template>
            {{ t('workflow.revisions.restoredFrom', { version: current.restoredFrom }) }}
          </a-tag>
        </div>
        <div class="trigger">
          <template v-if="current.path">
            <a-tag size="small" :color="METHOD_COLORS[current.method]" class="method">{{ current.method }}</a-tag>
            <code class="path">{{ current.path }}</code>
          </template>
          <template v-else-if="triggerMeta(current)">
            <span class="trigger-icon" :style="{ color: triggerMeta(current).color, background: `${triggerMeta(current).color}14` }"><component :is="triggerMeta(current).icon" :size="13" /></span>
            <span>{{ triggerMeta(current).label }}</span>
          </template>
        </div>
        <div class="current-meta">
          <template v-if="current.createdBy">
            <UserAvatar :name="current.createdBy.userName" :color="current.createdBy.color" :avatar-url="current.createdBy.avatarUrl" :size="18" />
            <span class="who">{{ current.createdBy.userName }}</span>
            <span class="sep" />
          </template>
          <a-tooltip :content="fullTime(current.createdAt)" mini>
            <span>{{ t('workflow.revisions.publishedAt', { time: dayjs(current.createdAt).fromNow() }) }}</span>
          </a-tooltip>
        </div>
        <div v-if="store.workflow?.hasUnpublishedChanges" class="draft-note">
          <span class="draft-dot" />
          {{ t('workflow.revisions.draftDirty') }}
        </div>
      </section>

      <section v-if="groups.length" class="history">
        <div class="section-title">
          <span>{{ t('workflow.revisions.history') }}</span>
          <a-tooltip :content="t('workflow.revisions.hint')" position="bl">
            <Info :size="13" class="info" />
          </a-tooltip>
        </div>

        <div v-for="g in groups" :key="g.key" class="group">
          <div class="group-label">{{ g.label }}</div>
          <ol class="rail">
            <li v-for="r in g.items" :key="r.version" class="row">
              <span class="node" />
              <span class="row-version">{{ t('workflow.editor.version', { version: r.version }) }}</span>
              <div class="row-main">
                <div class="row-line">
                  <template v-if="r.createdBy">
                    <UserAvatar :name="r.createdBy.userName" :color="r.createdBy.color" :avatar-url="r.createdBy.avatarUrl" :size="16" />
                    <span class="who">{{ r.createdBy.userName }}</span>
                  </template>
                  <a-tag v-if="r.restoredFrom" size="small" class="restored">
                    <template #icon><RotateCcw :size="10" /></template>
                    {{ t('workflow.revisions.restoredFrom', { version: r.restoredFrom }) }}
                  </a-tag>
                </div>
                <div v-if="triggerKey(r) !== triggerKey(current)" class="row-trigger">
                  <template v-if="r.path">
                    <span class="row-method" :style="{ color: `rgb(var(--${METHOD_COLORS[r.method] ?? 'gray'}-6))` }">{{ r.method }}</span>
                    <code>{{ r.path }}</code>
                  </template>
                  <span v-else-if="triggerMeta(r)">{{ triggerMeta(r).label }}</span>
                </div>
              </div>
              <div class="row-end">
                <a-tooltip :content="fullTime(r.createdAt)" mini>
                  <span class="row-time">{{ dayjs(r.createdAt).format('HH:mm') }}</span>
                </a-tooltip>
                <a-button v-if="store.canManage" size="mini" class="row-action" @click="rollback(r)">
                  <template #icon><RotateCcw :size="12" /></template>
                  {{ t('workflow.revisions.rollback') }}
                </a-button>
              </div>
            </li>
          </ol>
        </div>
      </section>
    </template>
  </a-drawer>
</template>

<style scoped>
.drawer-title {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}
.count {
  min-width: 20px;
  padding: 0 6px;
  border-radius: 10px;
  background: var(--fill-hover);
  color: var(--text-caption);
  font-size: 12px;
  font-weight: 500;
  line-height: 20px;
  text-align: center;
  font-variant-numeric: tabular-nums;
}

.skeleton {
  padding-top: 4px;
}
:deep(.skeleton-card) {
  width: 100%;
  height: 132px;
  margin-bottom: 28px;
  border-radius: var(--border-radius-large);
}

.empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 72px 24px;
}
.empty-title {
  margin-top: 8px;
  font-weight: 500;
  color: var(--text-title);
}
.empty-text {
  max-width: 280px;
  font-size: 13px;
  line-height: 1.6;
  color: var(--text-placeholder);
  text-align: center;
}

/* Current version card */
.current {
  padding: 16px;
  border: 1px solid var(--line-border);
  border-radius: var(--border-radius-large);
  background: var(--bg-body);
}
.current-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.eyebrow {
  font-size: 12px;
  font-weight: 500;
  color: var(--text-caption);
}
.status-dot {
  display: inline-block;
  width: 6px;
  height: 6px;
  margin-right: 6px;
  border-radius: 50%;
  background: currentColor;
  vertical-align: 1px;
}
.current-version {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 4px;
  font-size: 22px;
  font-weight: 600;
  line-height: 30px;
  color: var(--text-title);
  font-variant-numeric: tabular-nums;
}
.trigger {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 10px;
  font-size: 13px;
  color: var(--text-title);
}
.method {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-weight: 600;
}
.path {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 13px;
  overflow-wrap: anywhere;
}
.trigger-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border-radius: 6px;
}
.current-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid var(--line-border);
  font-size: 12px;
  color: var(--text-placeholder);
}
.who {
  color: var(--text-caption);
}
.sep {
  width: 3px;
  height: 3px;
  border-radius: 50%;
  background: var(--text-disabled);
}
.draft-note {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 10px;
  padding: 6px 10px;
  border-radius: var(--border-radius-small);
  background: rgba(var(--warning-6), 0.08);
  color: var(--color-warning);
  font-size: 12px;
}
.draft-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}
.restored {
  font-size: 11.5px;
  font-weight: 400;
  letter-spacing: 0;
  color: var(--text-caption);
}

/* History */
.history {
  margin-top: 28px;
}
.section-title {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 4px;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-title);
}
.info {
  color: var(--text-placeholder);
  cursor: help;
}
.group-label {
  margin: 14px 0 4px;
  font-size: 12px;
  color: var(--text-placeholder);
}
.rail {
  position: relative;
  margin: 0;
  padding: 0;
  list-style: none;
}
.rail::before {
  content: '';
  position: absolute;
  top: 18px;
  bottom: 18px;
  left: 11px;
  width: 1px;
  background: var(--line-border);
}
.row {
  position: relative;
  display: flex;
  align-items: flex-start;
  gap: 12px;
  margin: 0 -8px;
  padding: 9px 8px 9px 32px;
  border-radius: 8px;
  transition: background-color 0.15s;
}
.row:hover,
.row:focus-within {
  background: var(--bg-base);
}
.node {
  position: absolute;
  top: 15px;
  left: 15px;
  width: 9px;
  height: 9px;
  border: 2px solid var(--line-border-strong);
  border-radius: 50%;
  background: var(--bg-body);
  transition: border-color 0.15s;
}
.row:hover .node {
  border-color: var(--color-primary);
}
.row-version {
  min-width: 34px;
  font-weight: 600;
  line-height: 22px;
  color: var(--text-title);
  font-variant-numeric: tabular-nums;
}
.row-main {
  flex: 1;
  min-width: 0;
}
.row-line {
  display: flex;
  align-items: center;
  gap: 6px;
  min-height: 22px;
  font-size: 13px;
}
.row-trigger {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 2px;
  font-size: 12px;
  color: var(--text-caption);
}
.row-trigger code {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  overflow-wrap: anywhere;
}
.row-method {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-weight: 600;
}
.row-end {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  min-width: 72px;
  height: 22px;
}
.row-time {
  font-size: 12px;
  color: var(--text-placeholder);
  font-variant-numeric: tabular-nums;
  transition: opacity 0.15s;
}
.row-action {
  position: absolute;
  right: 0;
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.15s;
}
.row-action.arco-btn-secondary {
  border: 1px solid var(--line-border);
  background: var(--bg-body);
  color: var(--text-title);
  box-shadow: var(--shadow-card);
}
.row-action.arco-btn-secondary:hover {
  border-color: var(--color-primary);
  background: var(--bg-body);
  color: var(--color-primary);
}
.row:hover .row-action,
.row:focus-within .row-action {
  opacity: 1;
  pointer-events: auto;
}
.row:hover .row-time,
.row:focus-within .row-time {
  opacity: 0;
}

@media (hover: none) {
  .row-action {
    position: static;
    margin-left: 8px;
    opacity: 1;
    pointer-events: auto;
  }
}
</style>
