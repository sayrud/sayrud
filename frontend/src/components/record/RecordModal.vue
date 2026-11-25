<script setup lang="ts">
import { ChevronDown, ChevronRight, ChevronUp, Copy, Ellipsis, Plus, Trash, X } from '@lucide/vue'
import dayjs from 'dayjs'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import FieldValueEditor from '@/components/cell/FieldValueEditor.vue'
import MemberTag from '@/components/cell/MemberTag.vue'
import FieldTypeIcon from '@/components/field/FieldTypeIcon.vue'
import CommentPanel from '@/components/record/CommentPanel.vue'
import RecordHistory from '@/components/record/RecordHistory.vue'
import { openMenu } from '@/composables/useContextMenu'
import { visibleFieldsOf } from '@/composables/useViewData'
import { useBaseStore } from '@/stores/base'
import { isWriter } from '@/utils/directory'

/** Below this viewport width the comments move from the side into a tab. */
const NARROW_WIDTH = 1120

const { t } = useI18n()

const store = useBaseStore()

const state = computed(() => store.expandedRecord)
const record = computed(() => (state.value ? store.recordMap.get(state.value.uid) : undefined))
const list = computed(() => state.value?.list.filter((u) => store.recordMap.has(u)) ?? [])
const index = computed(() => (record.value ? list.value.indexOf(record.value.uid) : -1))

const visible = computed(() => visibleFieldsOf(store.fields, store.activeView))
const hidden = computed(() => store.fields.filter((f) => !visible.value.includes(f)))
const showHidden = ref(false)

const createdBy = computed(() => (isWriter(record.value?.createdBy) ? record.value!.createdBy! : null))
const updatedBy = computed(() => (isWriter(record.value?.updatedBy) ? record.value!.updatedBy! : null))

const title = computed(() => {
  const p = store.fields[0]
  return record.value && p ? store.ctx.text(record.value, p) : ''
})

// The public visitors see neither the comments nor the history.
const collaborative = computed(() => !store.isPublic)
const commentCount = computed(() => record.value?.commentCount ?? 0)

const narrow = ref(window.innerWidth < NARROW_WIDTH)
function onResize() {
  narrow.value = window.innerWidth < NARROW_WIDTH
}
const tab = ref<'detail' | 'history' | 'comments'>('detail')
watch(narrow, (value) => {
  if (!value && tab.value === 'comments') tab.value = 'detail'
})

const detailEl = ref<HTMLElement>()
const historyEl = ref<HTMLElement>()
// Switching records starts from the top of the new one.
watch(() => record.value?.uid, () => {
  void nextTick(() => {
    for (const el of [detailEl.value, historyEl.value]) if (el) el.scrollTop = 0
  })
})
watch(() => !!state.value, (open) => {
  if (!open) tab.value = 'detail'
})

function go(delta: number) {
  const next = list.value[index.value + delta]
  if (next && state.value) store.expandedRecord = { uid: next, list: state.value.list }
}

function close() {
  store.expandedRecord = null
}

function openMore(e: MouseEvent) {
  const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
  const uid = record.value?.uid
  if (!uid) return
  openMenu({ x: r.right - 180, y: r.bottom + 4 }, [
    {
      label: t('grid.duplicateRecord'),
      icon: Copy,
      onClick: async () => {
        const created = await store.duplicateRecord(uid)
        if (created && state.value) store.expandedRecord = { uid: created.uid, list: [...state.value.list, created.uid] }
      },
    },
    { divider: true },
    { label: t('grid.deleteRecord'), icon: Trash, danger: true, onClick: () => store.deleteRecords([uid]) },
  ])
}

function addField(e: MouseEvent) {
  const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
  store.openFieldEditor({ mode: 'create', anchor: { x: r.left, y: r.top, width: r.width, height: r.height } })
}

function onKey(e: KeyboardEvent) {
  if (!state.value) return
  const t = e.target as HTMLElement
  if (t.closest('input, textarea, [contenteditable]')) return
  if (e.key === 'ArrowUp' || e.key === 'k') go(-1)
  if (e.key === 'ArrowDown' || e.key === 'j') go(1)
}
onMounted(() => {
  document.addEventListener('keydown', onKey)
  window.addEventListener('resize', onResize)
})
onBeforeUnmount(() => {
  document.removeEventListener('keydown', onKey)
  window.removeEventListener('resize', onResize)
})
</script>

<template>
  <a-modal
    :visible="!!record"
    :width="collaborative && !narrow ? 1080 : 760"
    :footer="false"
    :closable="false"
    :body-style="{ padding: 0 }"
    modal-class="record-modal"
    unmount-on-close
    @cancel="close"
  >
    <template #title>
      <div class="head">
        <div class="nav">
          <button class="icon-btn" :disabled="index <= 0" :title="t('record.previous')" @click="go(-1)"><ChevronUp :size="16" /></button>
          <button class="icon-btn" :disabled="index < 0 || index >= list.length - 1" :title="t('record.next')" @click="go(1)">
            <ChevronDown :size="16" />
          </button>
          <span v-if="index >= 0" class="pos">{{ index + 1 }} / {{ list.length }}</span>
        </div>
        <span class="spacer" />
        <button v-if="store.canEdit" class="icon-btn" @click="openMore"><Ellipsis :size="16" /></button>
        <button class="icon-btn" @click="close"><X :size="16" /></button>
      </div>
    </template>

    <div v-if="record" class="layout" :class="{ split: collaborative && !narrow, solo: !collaborative }">
      <div class="main">
        <h2 class="title" :class="{ empty: !title }">{{ title || t('record.untitled') }}</h2>
        <a-tabs v-model:active-key="tab" class="tabs" lazy-load>
          <a-tab-pane key="detail" :title="t('record.tabDetail')">
            <div ref="detailEl" class="pane">
              <div v-for="f in visible" :key="f.uid" class="field-row">
                <div class="field-label">
                  <FieldTypeIcon :type="f.type" />
                  <span class="ellipsis">{{ f.label }}</span>
                </div>
                <div class="field-value">
                  <FieldValueEditor
                    :field="f"
                    :value="record.data[f.uid]"
                    :record="record"
                    :placeholder="t('group.empty')"
                    :readonly="!store.canEdit"
                    @change="(v) => store.updateCell(record!.uid, f.uid, v)"
                  />
                </div>
              </div>

              <template v-if="hidden.length">
                <button class="toggle-hidden" @click="showHidden = !showHidden">
                  <component :is="showHidden ? ChevronDown : ChevronRight" :size="14" />
                  {{ t('toolbar.hiddenFields', { n: hidden.length }, hidden.length) }}
                </button>
                <template v-if="showHidden">
                  <div v-for="f in hidden" :key="f.uid" class="field-row">
                    <div class="field-label">
                      <FieldTypeIcon :type="f.type" />
                      <span class="ellipsis">{{ f.label }}</span>
                    </div>
                    <div class="field-value">
                      <FieldValueEditor
                        :field="f"
                        :value="record.data[f.uid]"
                        :record="record"
                        :placeholder="t('group.empty')"
                        :readonly="!store.canEdit"
                        @change="(v) => store.updateCell(record!.uid, f.uid, v)"
                      />
                    </div>
                  </div>
                </template>
              </template>

              <button v-if="store.canEdit" class="add-field" @click="addField"><Plus :size="14" /> {{ t('grid.addField') }}</button>

              <div class="meta">
                <span class="meta-item">
                  <MemberTag v-if="createdBy" :user-id="createdBy.userId" :workflow-id="createdBy.workflowId" small />
                  {{ t('record.createdAt', { time: dayjs(record.createdAt).format('YYYY/MM/DD HH:mm') }) }}
                </span>
                <span class="meta-item">
                  <MemberTag v-if="updatedBy" :user-id="updatedBy.userId" :workflow-id="updatedBy.workflowId" small />
                  {{ t('record.updatedAt', { time: dayjs(record.updatedAt).format('YYYY/MM/DD HH:mm') }) }}
                </span>
                <span class="meta-item">{{ record.uid }}</span>
              </div>
            </div>
          </a-tab-pane>
          <template v-if="collaborative">
            <a-tab-pane key="history" :title="t('record.tabHistory')">
              <div ref="historyEl" class="pane"><RecordHistory :record="record" /></div>
            </a-tab-pane>
            <a-tab-pane v-if="narrow" key="comments">
              <template #title>
                {{ t('comments.title') }}<span v-if="commentCount" class="count">{{ commentCount }}</span>
              </template>
              <CommentPanel class="tab-comments" :record-uid="record.uid" />
            </a-tab-pane>
          </template>
        </a-tabs>
      </div>
      <aside v-if="collaborative && !narrow" class="side">
        <div class="side-head">
          {{ t('comments.title') }}<span v-if="commentCount" class="count">{{ commentCount }}</span>
        </div>
        <CommentPanel class="side-comments" :record-uid="record.uid" />
      </aside>
    </div>
  </a-modal>
</template>

<style scoped>
.head {
  display: flex;
  align-items: center;
  width: 100%;
  gap: 4px;
}
.nav {
  display: flex;
  align-items: center;
  gap: 2px;
}
.pos {
  margin-left: 6px;
  font-size: 13px;
  font-weight: normal;
  color: var(--text-placeholder);
}
.spacer {
  flex: 1;
}
.layout {
  display: flex;
  height: min(72vh, 780px);
}
.layout.solo {
  height: auto;
  max-height: 72vh;
}
.main {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
  padding: 4px 0 0;
}
.title {
  margin: 4px 32px 8px;
  font-size: 22px;
  font-weight: 600;
  word-break: break-all;
}
.title.empty {
  color: var(--text-placeholder);
}
/* The tabs fill the column, and each pane scrolls on its own. */
.tabs {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
}
.tabs :deep(.arco-tabs-nav) {
  padding: 0 24px;
}
.tabs :deep(.arco-tabs-content) {
  flex: 1;
  min-height: 0;
  padding-top: 0;
}
.tabs :deep(.arco-tabs-content-list),
.tabs :deep(.arco-tabs-pane) {
  height: 100%;
}
.layout.solo .tabs :deep(.arco-tabs-nav) {
  display: none;
}
.pane {
  height: 100%;
  overflow: auto;
  padding: 16px 32px 20px;
}
.layout.solo .pane {
  max-height: calc(72vh - 60px);
}
.count {
  margin-left: 6px;
  padding: 0 6px;
  border-radius: 9px;
  font-size: 12px;
  line-height: 18px;
  font-weight: 500;
  color: var(--color-text-2);
  background: var(--color-fill-2);
}
.side {
  display: flex;
  flex-direction: column;
  width: 360px;
  flex: none;
  border-left: 1px solid var(--color-border-2);
  background: var(--color-bg-2);
}
.side-head {
  display: flex;
  align-items: center;
  height: 44px;
  padding: 0 16px;
  flex: none;
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text-1);
  border-bottom: 1px solid var(--color-border-1);
}
.side-comments {
  flex: 1;
  min-height: 0;
}
.tab-comments {
  height: 100%;
}
.field-row {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 14px;
}
.field-label {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 150px;
  flex: none;
  height: 32px;
  color: var(--text-caption);
  font-size: 13px;
}
.field-value {
  flex: 1;
  min-width: 0;
}
.toggle-hidden,
.add-field {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin: 4px 0 12px;
  padding: 4px 6px;
  border: none;
  background: transparent;
  color: var(--text-caption);
  border-radius: 6px;
  cursor: pointer;
}
.toggle-hidden:hover,
.add-field:hover {
  background: var(--fill-hover);
}
.add-field {
  display: flex;
  color: var(--color-primary);
}
.meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px 16px;
  margin-top: 16px;
  padding-top: 12px;
  border-top: 1px solid var(--line-divider);
  font-size: 12px;
  color: var(--text-placeholder);
}
.meta-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}
</style>

<style>
.record-modal .arco-modal-header {
  border-bottom: none;
  padding: 0 16px;
}
.record-modal .arco-modal-title {
  width: 100%;
}
</style>
