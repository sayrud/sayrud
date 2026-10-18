<script setup lang="ts">
import { Message, Modal, type TableColumnData } from '@arco-design/web-vue'
import { PanelLeftOpen, Plus } from '@lucide/vue'
import dayjs from 'dayjs'
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { credentialsApi, workflowsApi } from '@/api/workflow'
import StateIllustration from '@/components/common/StateIllustration.vue'
import PageHeader from '@/components/console/PageHeader.vue'
import SettingsSection from '@/components/console/SettingsSection.vue'
import CredentialDialog from '@/components/workflow/module/CredentialDialog.vue'
import type { Credential, CredentialTypeDef, FieldDef } from '@/types/workflow'
import { credentialTypeDef, dslText, moduleIcon, setModuleDefs } from '@/utils/modules'

const props = defineProps<{ projectUid: string; canManage: boolean; sidebarCollapsed?: boolean }>()
const emit = defineEmits<{ expandSidebar: [] }>()
const { t } = useI18n()

const credentials = ref<Credential[]>([])
const types = ref<CredentialTypeDef[]>([])
const loading = ref(true)
const loadError = ref('')
const keyword = ref('')
const type = ref<string>()
const pagination = reactive({ current: 1, pageSize: 20 })

const selected = ref<Credential | null>(null)
const detailsVisible = ref(false)
const detailsLoading = ref(false)
const detailsError = ref('')
const opening = ref('')
const editing = ref<Credential | null>(null)
const editingType = ref('')
const dialogVisible = ref(false)

const columns = computed<TableColumnData[]>(() => [
  { title: t('workflow.credential.name'), slotName: 'name', width: 320 },
  { title: t('credentialPage.type'), slotName: 'type', width: 180 },
  { title: t('credentialPage.updatedAt'), slotName: 'updatedAt', width: 160 },
  { title: t('common.actions'), slotName: 'actions', width: props.canManage ? 180 : 80, fixed: 'right' },
])

function summary(c: Credential) {
  return (credentialTypeDef(c.type)?.summary ?? []).map((key) => c.data[key]).filter((value) => value !== undefined && value !== '').join(' · ')
}

const filtered = computed(() => {
  const search = keyword.value.trim().toLowerCase()
  return credentials.value.filter((c) => (!type.value || c.type === type.value) && (!search || [c.name, dslText(credentialTypeDef(c.type)?.label), c.type, summary(c)].some((value) => value.toLowerCase().includes(search))))
})
const paged = computed(() => filtered.value.slice((pagination.current - 1) * pagination.pageSize, pagination.current * pagination.pageSize))
const typeOptions = computed(() => [...new Set([...types.value.map((def) => def.type), ...credentials.value.map((c) => c.type)])].map((value) => ({ value, label: dslText(credentialTypeDef(value)?.label) || value })))

watch([keyword, type, () => pagination.pageSize], () => { pagination.current = 1 })
watch(() => filtered.value.length, (total) => {
  pagination.current = Math.min(pagination.current, Math.max(1, Math.ceil(total / pagination.pageSize)))
})

let listRequest = 0
let detailRequest = 0

async function load() {
  const request = ++listRequest
  loading.value = true
  loadError.value = ''

  try {
    const [list, definitions] = await Promise.all([credentialsApi.list(props.projectUid), workflowsApi.modules(props.projectUid)])
    if (request !== listRequest) return
    setModuleDefs(definitions)
    credentials.value = list
    types.value = definitions.credentialTypes
  } catch (e) {
    if (request === listRequest) loadError.value = e instanceof Error ? e.message : String(e)
  } finally {
    if (request === listRequest) loading.value = false
  }
}

function create(def: CredentialTypeDef) {
  if (!props.canManage) return
  editing.value = null
  editingType.value = def.type
  dialogVisible.value = true
}

function edit(c: Credential) {
  if (!props.canManage || !credentialTypeDef(c.type)) return
  closeDetails()
  editing.value = c
  editingType.value = c.type
  dialogVisible.value = true
}

async function inspect(c: Credential, forEditing = false) {
  if (forEditing && (!props.canManage || opening.value)) return
  const request = ++detailRequest
  if (forEditing) opening.value = c.uid
  else {
    selected.value = null
    detailsError.value = ''
    detailsLoading.value = true
    detailsVisible.value = true
  }

  try {
    const current = await credentialsApi.get(props.projectUid, c.uid)
    if (request !== detailRequest) return
    if (forEditing) edit(current)
    else selected.value = current
  } catch (e) {
    if (request !== detailRequest) return
    const message = e instanceof Error ? e.message : String(e)
    if (forEditing) Message.error(message)
    else detailsError.value = message
  } finally {
    if (request === detailRequest) detailsLoading.value = false
    if (opening.value === c.uid) opening.value = ''
  }
}

function closeDetails() {
  detailRequest++
  detailsVisible.value = false
  detailsLoading.value = false
}

function onSaved(c: Credential) {
  const index = credentials.value.findIndex((item) => item.uid === c.uid)
  if (index >= 0) credentials.value.splice(index, 1, c)
  else {
    credentials.value.unshift(c)
    keyword.value = ''
    type.value = undefined
    pagination.current = 1
  }
  if (selected.value?.uid === c.uid) selected.value = c
}

function onDeleted(uid: string) {
  credentials.value = credentials.value.filter((c) => c.uid !== uid)
  if (selected.value?.uid === uid) closeDetails()
}

function remove(c: Credential) {
  if (!props.canManage) return
  Modal.confirm({
    title: t('workflow.credential.deleteTitle', { name: c.name }),
    content: t('workflow.credential.deleteContent'),
    okText: t('common.delete'),
    okButtonProps: { status: 'danger' },
    onBeforeOk: async () => {
      try {
        await credentialsApi.remove(props.projectUid, c.uid)
        onDeleted(c.uid)
        Message.success(t('common.deleted'))
        return true
      } catch (e) {
        Message.error(e instanceof Error ? e.message : String(e))
        return false
      }
    },
  })
}

const detailDef = computed(() => selected.value ? credentialTypeDef(selected.value.type) : undefined)
const detailFields = computed<FieldDef[]>(() => detailDef.value?.sections.filter((s) => !s.showIfValue || selected.value?.data[s.showIf!] === s.showIfValue).flatMap((section) => section.fields) ?? [
  ...Object.keys(selected.value?.data ?? {}).map((key) => ({ key, label: key, widget: 'text' as const })),
  ...(selected.value?.secrets ?? []).map((key) => ({ key, label: key, widget: 'text' as const, secret: true })),
])

function fieldValue(field: FieldDef) {
  const value = selected.value?.data[field.key!]
  const option = field.options?.find((item) => item.value === value)
  if (option?.label) return dslText(option.label)
  if (value === undefined || value === null || value === '') return '—'
  return typeof value === 'object' ? JSON.stringify(value) : String(value)
}

const formatTime = (time: string) => dayjs(time).format('YYYY-MM-DD HH:mm')

onMounted(load)
onBeforeUnmount(() => { listRequest++; detailRequest++ })
</script>

<template>
  <section class="credentials-page">
    <div class="credentials-content">
      <div class="page-top">
        <a-button v-if="sidebarCollapsed" type="text" class="expand-sidebar" :aria-label="t('base.expandSidebar')" @click="emit('expandSidebar')">
          <template #icon><PanelLeftOpen :size="18" /></template>
        </a-button>
        <PageHeader :title="t('credentialPage.title')" :description="t('credentialPage.description')">
          <template v-if="canManage" #extra>
            <a-dropdown trigger="click" position="br">
              <a-button type="primary" :disabled="loading || !!loadError || !types.length">
                <template #icon><Plus :size="16" /></template>
                {{ t('workflow.credential.create') }}
              </a-button>
              <template #content>
                <a-doption v-for="def in types" :key="def.type" @click="create(def)">
                  <template #icon><component :is="moduleIcon(def.icon)" :size="16" /></template>
                  {{ dslText(def.label) }}
                </a-doption>
              </template>
            </a-dropdown>
          </template>
        </PageHeader>
      </div>

      <a-alert v-if="!canManage" type="info" class="readonly-hint">{{ t('credentialPage.readonly') }}</a-alert>

      <a-result v-if="loadError" status="error" :title="t('credentialPage.loadFailed')" :subtitle="loadError">
        <template #extra><a-button @click="load">{{ t('common.retry') }}</a-button></template>
      </a-result>

      <SettingsSection v-else flush>
        <div v-if="credentials.length || keyword.trim() || type" class="toolbar">
          <a-select v-model="type" :options="typeOptions" :placeholder="t('credentialPage.allTypes')" allow-clear class="type-filter" :aria-label="t('credentialPage.type')" />
          <a-input-search v-model="keyword" :placeholder="t('credentialPage.search')" allow-clear class="search" />
        </div>

        <a-table
          v-if="loading || credentials.length || keyword.trim() || type"
          :columns="columns"
          :data="paged"
          :loading="loading"
          :show-header="loading || filtered.length > 0"
          row-key="uid"
          :bordered="false"
          :pagination="false"
          :scroll="{ x: canManage ? 840 : 740 }"
          class="credentials-table"
          @row-click="(record) => inspect(record as Credential)"
        >
          <template #name="{ record }">
            <div class="name-cell">
              <a-avatar shape="square" :size="36" class="credential-icon"><component :is="moduleIcon(credentialTypeDef(record.type)?.icon || 'key')" :size="18" /></a-avatar>
              <div class="name-text">
                <a-link class="name" @click.stop="inspect(record as Credential)">{{ record.name }}</a-link>
                <div class="text-desc ellipsis">{{ summary(record as Credential) || t('credentialPage.projectShared') }}</div>
              </div>
            </div>
          </template>
          <template #type="{ record }">
            <a-tag :color="credentialTypeDef(record.type) ? undefined : 'orange'">{{ dslText(credentialTypeDef(record.type)?.label) || record.type }}</a-tag>
          </template>
          <template #updatedAt="{ record }">{{ formatTime(record.updatedAt) }}</template>
          <template #actions="{ record }">
            <a-space :size="0" @click.stop>
              <a-button type="text" size="small" @click="inspect(record as Credential)">{{ t('credentialPage.view') }}</a-button>
              <template v-if="canManage">
                <a-tooltip :content="credentialTypeDef(record.type) ? t('workflow.credential.edit') : t('credentialPage.typeUnavailable')">
                  <a-button type="text" size="small" :disabled="!credentialTypeDef(record.type) || !!opening" :loading="opening === record.uid" @click="inspect(record as Credential, true)">{{ t('common.edit') }}</a-button>
                </a-tooltip>
                <a-button type="text" status="danger" size="small" @click="remove(record as Credential)">{{ t('common.delete') }}</a-button>
              </template>
            </a-space>
          </template>
          <template #empty>
            <a-empty :description="t('credentialPage.noMatch')" class="no-results">
              <template #image><StateIllustration name="no-results" :width="140" /></template>
            </a-empty>
          </template>
        </a-table>

        <div v-else class="empty">
          <StateIllustration name="feature-upgrade" :width="160" />
          <div class="empty-title">{{ t('credentialPage.empty') }}</div>
          <div class="text-desc empty-description">{{ t('credentialPage.emptyDescription') }}</div>
          <a-dropdown v-if="canManage" trigger="click" position="bottom">
            <a-button type="primary" :disabled="!types.length">
              <template #icon><Plus :size="16" /></template>
              {{ t('workflow.credential.create') }}
            </a-button>
            <template #content>
              <a-doption v-for="def in types" :key="def.type" @click="create(def)">
                <template #icon><component :is="moduleIcon(def.icon)" :size="16" /></template>
                {{ dslText(def.label) }}
              </a-doption>
            </template>
          </a-dropdown>
        </div>

        <div v-if="credentials.length" class="pagination-footer">
          <a-pagination v-model:current="pagination.current" v-model:page-size="pagination.pageSize" :total="filtered.length" :page-size-options="[10, 20, 50, 100]" show-total show-page-size :show-jumper="filtered.length > pagination.pageSize">
            <template #total="{ total }">{{ t('credentialPage.total', { n: total }, total) }}</template>
          </a-pagination>
        </div>
      </SettingsSection>
    </div>

    <a-drawer v-model:visible="detailsVisible" width="min(520px, 100vw)" :title="t('credentialPage.details')" :body-style="{ padding: '24px' }" unmount-on-close @cancel="closeDetails">
      <a-spin :loading="detailsLoading" class="detail-content">
        <a-alert v-if="detailsError" type="error">{{ detailsError }}</a-alert>
        <a-space v-else-if="selected" direction="vertical" size="large" fill>
          <a-space size="medium" fill>
            <a-avatar shape="square" :size="48" class="credential-icon"><component :is="moduleIcon(detailDef?.icon || 'key')" :size="24" /></a-avatar>
            <a-space direction="vertical" size="mini">
              <a-typography-title :heading="5" class="detail-title">{{ selected.name }}</a-typography-title>
              <a-typography-text type="secondary">{{ dslText(detailDef?.label) || selected.type }}</a-typography-text>
            </a-space>
          </a-space>

          <a-divider :margin="0" />

          <a-alert v-if="!detailDef" type="warning">{{ t('credentialPage.typeUnavailable') }}</a-alert>

          <a-space direction="vertical" size="medium" fill>
            <a-descriptions
              :column="1"
              :title="t('credentialPage.configuration')"
              size="medium"
              bordered
              table-layout="fixed"
              :label-style="{ width: '140px', whiteSpace: 'normal', overflowWrap: 'anywhere' }"
            >
              <a-descriptions-item v-for="field in detailFields" :key="field.key" :label="dslText(field.label) || field.key">
                <a-typography-text v-if="field.secret" type="secondary" :aria-label="selected.secrets.includes(field.key!) ? t('credentialPage.secretSaved') : undefined">
                  {{ selected.secrets.includes(field.key!) ? '••••••••' : t('credentialPage.secretEmpty') }}
                </a-typography-text>
                <template v-else>{{ fieldValue(field) }}</template>
              </a-descriptions-item>
            </a-descriptions>

            <a-alert v-if="detailFields.some((field) => field.secret)" type="normal">{{ t('credentialPage.secretHint') }}</a-alert>
          </a-space>

          <a-divider :margin="0" />

          <a-descriptions :column="{ xs: 1, sm: 2 }" layout="vertical" table-layout="fixed">
            <a-descriptions-item :label="t('credentialPage.createdAt')">{{ formatTime(selected.createdAt) }}</a-descriptions-item>
            <a-descriptions-item :label="t('credentialPage.updatedAt')">{{ formatTime(selected.updatedAt) }}</a-descriptions-item>
          </a-descriptions>
        </a-space>
      </a-spin>
      <template #footer>
        <a-button @click="closeDetails">{{ t('common.close') }}</a-button>
        <a-button v-if="canManage" type="primary" :disabled="!selected || detailsLoading || !detailDef" @click="selected && edit(selected)">{{ t('workflow.credential.edit') }}</a-button>
      </template>
    </a-drawer>

    <CredentialDialog v-if="canManage && editingType" v-model:visible="dialogVisible" :project-uid="projectUid" :type="editingType" :credential="editing" @saved="onSaved" @deleted="onDeleted" />
  </section>
</template>

<style scoped>
.credentials-page {
  flex: 1;
  min-height: 0;
  overflow: auto;
  background: var(--bg-base);
}
.credentials-content {
  max-width: 1080px;
  margin: 0 auto;
  padding: 24px 32px 48px;
}
.page-top {
  display: flex;
  align-items: flex-start;
  gap: 12px;
}
.page-top :deep(.page-header) {
  flex: 1;
  min-width: 0;
}
.expand-sidebar {
  flex: none;
  margin-top: 2px;
  color: var(--text-caption);
}
.readonly-hint {
  margin-bottom: 20px;
}
.toolbar {
  display: flex;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;
  padding: 20px 24px;
}
.toolbar :deep(.type-filter) {
  width: 180px;
}
.search {
  width: 280px;
}
.name-cell {
  display: flex;
  align-items: center;
  gap: 12px;
}
.credential-icon {
  flex: none;
  color: var(--color-primary);
  background: var(--bg-primary-soft);
  border-radius: 8px;
}
.name-text {
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.name {
  align-self: flex-start;
  max-width: 100%;
  padding: 0;
  color: var(--text-title);
  font-weight: 500;
}
.credentials-table :deep(.arco-table-tr) {
  cursor: pointer;
}
.credentials-table :deep(> .arco-spin) {
  height: auto;
}
.credentials-table :deep(.arco-table-th) {
  background: var(--bg-base);
}
.empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 56px 24px;
  text-align: center;
}
.empty-title {
  font-size: 15px;
  font-weight: 500;
  color: var(--text-title);
}
.no-results {
  padding: 40px 20px;
}
.empty-description {
  max-width: 420px;
  margin-bottom: 12px;
}
.pagination-footer {
  padding: 16px 24px;
}
.pagination-footer :deep(.arco-pagination) {
  width: 100%;
  flex-wrap: wrap;
  row-gap: 12px;
}
.pagination-footer :deep(.arco-pagination-total) {
  flex: 1;
  color: var(--text-caption);
}
.detail-content {
  display: block;
  width: 100%;
  min-height: 160px;
}
.detail-title.arco-typography {
  margin: 0;
}
@media (max-width: 768px) {
  .credentials-content {
    padding: 16px 16px 40px;
  }
  .toolbar {
    padding: 16px;
  }
  .search {
    flex: 1;
    min-width: 180px;
  }
  .toolbar :deep(.type-filter) {
    width: 160px;
  }
  .pagination-footer {
    padding: 16px;
  }
  .pagination-footer :deep(.arco-pagination-total) {
    flex-basis: 100%;
  }
}
</style>
