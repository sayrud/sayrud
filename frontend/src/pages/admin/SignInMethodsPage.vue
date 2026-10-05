<script setup lang="ts">
import { Message, Modal } from '@arco-design/web-vue'
import { ArrowDown, ArrowUp, Plus } from '@lucide/vue'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { ssoApi, type AdminAuthProvider } from '@/api/sso'
import AuthProviderDrawer from '@/components/admin/AuthProviderDrawer.vue'
import ProviderIcon from '@/components/common/ProviderIcon.vue'
import PageHeader from '@/components/console/PageHeader.vue'
import SettingRow from '@/components/console/SettingRow.vue'
import SettingsSection from '@/components/console/SettingsSection.vue'
import { useSystemSettings } from '@/composables/useSystemSettings'
import { useSiteStore } from '@/stores/site'
import { TEMPLATES, draftFromTemplate, type ProviderDraft, type ProviderTemplate } from '@/utils/ssoTemplates'

const { t } = useI18n()
const site = useSiteStore()
const {
  draft: settingsDraft,
  loading: settingsLoading,
  saving: settingsSaving,
  save: saveSettings,
  reset: resetSettings,
} = useSystemSettings()

const TYPE_LABELS: Record<AdminAuthProvider['type'], string> = { oauth2: 'OAuth 2.0', oidc: 'OIDC', saml: 'SAML', ldap: 'LDAP' }

const providers = ref<AdminAuthProvider[]>([])
const secretsReady = ref(true)
const externalURL = ref('')
const loading = ref(true)
const busy = ref(false)

const pickerVisible = ref(false)
const drawerVisible = ref(false)
const editing = ref<AdminAuthProvider | null>(null)
const initial = ref<ProviderDraft | null>(null)

const GENERIC_KEYS = ['oidc', 'oauth2', 'saml', 'ldap']

// Generic cards already name their protocol, so they omit the protocol badge.
const pickerGroups = computed(() => [
  { title: t('sso.admin.templatePreset'), showType: true, templates: TEMPLATES.filter((tpl) => !GENERIC_KEYS.includes(tpl.key)) },
  { title: t('sso.admin.templateGeneric'), showType: false, templates: TEMPLATES.filter((tpl) => GENERIC_KEYS.includes(tpl.key)) },
])

async function load() {
  try {
    const resp = await ssoApi.providers()
    providers.value = resp.providers
    secretsReady.value = resp.secretsReady
    externalURL.value = resp.externalURL
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    loading.value = false
  }
}

function pick(template: ProviderTemplate) {
  pickerVisible.value = false
  editing.value = null
  initial.value = draftFromTemplate(
    template,
    providers.value.map((p) => p.slug),
  )
  drawerVisible.value = true
}

function edit(p: AdminAuthProvider) {
  editing.value = p
  initial.value = null
  drawerVisible.value = true
}

async function onSaved() {
  await Promise.all([load(), site.refresh()])
}

function updateBody(p: AdminAuthProvider, enabled: boolean) {
  return {
    name: p.name,
    icon: p.icon,
    enabled,
    config: p.config,
    autoCreateUser: p.autoCreateUser,
    linkByEmail: p.linkByEmail,
    allowedEmailDomains: p.allowedEmailDomains,
    allowedGroups: p.allowedGroups,
  }
}

async function toggle(p: AdminAuthProvider, enabled: boolean) {
  busy.value = true
  try {
    const saved = await ssoApi.update(p.id, updateBody(p, enabled))
    providers.value = providers.value.map((x) => (x.id === saved.id ? saved : x))
    await site.refresh()
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    busy.value = false
  }
}

async function move(index: number, delta: number) {
  const list = [...providers.value]
  const target = index + delta
  if (target < 0 || target >= list.length) return
  ;[list[index], list[target]] = [list[target]!, list[index]!]
  busy.value = true
  try {
    await ssoApi.reorder(list.map((p) => p.id))
    providers.value = list
    await site.refresh()
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    busy.value = false
  }
}

function remove(p: AdminAuthProvider) {
  Modal.confirm({
    title: t('sso.admin.deleteTitle', { name: p.name }),
    content: t('sso.admin.deleteContent', { n: p.identityCount }),
    okButtonProps: { status: 'danger' },
    okText: t('common.delete'),
    onBeforeOk: async () => {
      try {
        await ssoApi.remove(p.id)
      } catch (e) {
        Message.error(e instanceof Error ? e.message : String(e))
        return false
      }
      providers.value = providers.value.filter((x) => x.id !== p.id)
      await site.refresh()
      Message.success(t('common.deleted'))
      return true
    },
  })
}

async function togglePassword(value: string | number | boolean) {
  settingsDraft.allowPasswordSignIn = !!value
  if (!(await saveSettings())) resetSettings()
}

onMounted(load)
</script>

<template>
  <div>
    <PageHeader :title="t('sso.admin.title')" :description="t('sso.admin.description')">
      <template #extra>
        <a-button type="primary" :disabled="!secretsReady" @click="pickerVisible = true">
          <template #icon><Plus :size="16" /></template>
          {{ t('sso.admin.add') }}
        </a-button>
      </template>
    </PageHeader>

    <a-alert v-if="!loading && !secretsReady" type="warning" class="alert">{{ t('sso.admin.secretsMissing') }}</a-alert>
    <a-alert v-if="!loading && !externalURL" type="warning" class="alert">
      {{ t('sso.admin.externalMissing') }}
      <template #action>
        <router-link :to="{ name: 'admin-site' }">
          <a-button size="mini" type="text">{{ t('sso.admin.goSite') }}</a-button>
        </router-link>
      </template>
    </a-alert>

    <a-spin :loading="loading || settingsLoading" class="spin">
      <SettingsSection :title="t('sso.admin.passwordSection')">
        <SettingRow :label="t('sso.admin.allowPasswordSignIn')" :description="t('sso.admin.allowPasswordSignInDescription')">
          <template #action>
            <a-switch :model-value="settingsDraft.allowPasswordSignIn" :loading="settingsSaving" @change="togglePassword" />
          </template>
        </SettingRow>
      </SettingsSection>

      <SettingsSection :title="t('sso.admin.methods')" flush>
        <div v-if="providers.length" class="list">
          <div v-for="(p, index) in providers" :key="p.id" class="item">
            <ProviderIcon :icon="p.icon" :size="24" />
            <div class="item-text">
              <div class="item-name">
                <span class="ellipsis">{{ p.name }}</span>
                <a-tag size="small">{{ TYPE_LABELS[p.type] }}</a-tag>
                <a-tag v-if="!p.enabled" size="small" color="gray">{{ t('sso.admin.disabledTag') }}</a-tag>
              </div>
              <div class="text-desc ellipsis">{{ p.slug }} · {{ t('sso.admin.identityCount', { n: p.identityCount }) }}</div>
            </div>
            <a-space class="item-actions">
              <a-switch :model-value="p.enabled" size="small" :disabled="busy" @change="(v) => toggle(p, !!v)" />
              <a-button size="small" type="text" shape="square" :disabled="busy || index === 0" :aria-label="t('sso.admin.moveUp')" @click="move(index, -1)">
                <template #icon><ArrowUp :size="14" /></template>
              </a-button>
              <a-button
                size="small"
                type="text"
                shape="square"
                :disabled="busy || index === providers.length - 1"
                :aria-label="t('sso.admin.moveDown')"
                @click="move(index, 1)"
              >
                <template #icon><ArrowDown :size="14" /></template>
              </a-button>
              <a-button size="small" type="text" @click="edit(p)">{{ t('common.edit') }}</a-button>
              <a-button size="small" type="text" status="danger" @click="remove(p)">{{ t('common.delete') }}</a-button>
            </a-space>
          </div>
        </div>
        <a-empty v-else-if="!loading" :description="t('sso.admin.empty')" class="empty" />
      </SettingsSection>
    </a-spin>

    <a-modal
      v-model:visible="pickerVisible"
      :title="t('sso.admin.pickTemplate')"
      title-align="start"
      :footer="false"
      :width="880"
      modal-class="provider-picker-modal"
    >
      <template v-for="group in pickerGroups" :key="group.title">
        <div class="picker-group">{{ group.title }}</div>
        <div class="picker">
          <button v-for="tpl in group.templates" :key="tpl.key" type="button" class="picker-card" @click="pick(tpl)">
            <span v-if="group.showType" class="picker-badge">{{ TYPE_LABELS[tpl.type] }}</span>
            <span class="picker-head">
              <ProviderIcon :icon="tpl.icon" :size="24" />
              <span class="picker-name ellipsis">{{ tpl.name }}</span>
            </span>
            <span class="picker-desc">{{ t(`sso.admin.templateDesc.${tpl.key}`) }}</span>
          </button>
        </div>
      </template>
    </a-modal>

    <AuthProviderDrawer v-model:visible="drawerVisible" :provider="editing" :initial="initial" :site-url="externalURL" @saved="onSaved" />
  </div>
</template>

<style scoped>
.alert {
  margin-bottom: 16px;
}
.spin {
  display: block;
}
.list {
  display: flex;
  flex-direction: column;
}
.item {
  display: flex;
  gap: 12px;
  align-items: center;
  padding: 14px 24px;
}
.item + .item {
  border-top: 1px solid var(--line-border);
}
.item-text {
  flex: 1;
  min-width: 0;
}
.item-name {
  display: flex;
  gap: 8px;
  align-items: center;
  color: var(--text-title);
}
.item-actions {
  flex: none;
}
.empty {
  padding: 32px 0;
}
.picker-group {
  margin-bottom: 12px;
  font-size: 13px;
  color: var(--text-caption);
}
.picker {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
}
.picker + .picker-group {
  margin-top: 24px;
}
.picker-card {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 22px 20px;
  overflow: hidden;
  text-align: left;
  cursor: pointer;
  background: var(--bg-body);
  border: none;
  border-radius: 8px;
  font: inherit;
  transition: box-shadow 0.15s;
}
/* Draw the hover outline outside the card to keep the corner badge edges intact. */
.picker-card:hover {
  box-shadow:
    0 0 0 1px rgb(var(--primary-6)),
    var(--shadow-card);
}
.picker-card:focus-visible {
  outline: 2px solid rgb(var(--primary-6));
  outline-offset: 2px;
}
.picker-badge {
  position: absolute;
  top: 0;
  right: 0;
  padding: 1px 8px;
  font-size: 12px;
  line-height: 20px;
  color: rgb(var(--primary-6));
  background: var(--bg-primary-soft);
  border-bottom-left-radius: 8px;
}
.picker-head {
  display: flex;
  gap: 10px;
  align-items: center;
}
.picker-name {
  flex: 1;
  min-width: 0;
  font-size: 16px;
  font-weight: 500;
  color: var(--text-title);
}
.picker-desc {
  display: -webkit-box;
  min-height: calc(13px * 1.6 * 2);
  overflow: hidden;
  font-size: 13px;
  line-height: 1.6;
  color: var(--text-caption);
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}
@media (max-width: 900px) {
  .picker {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
@media (max-width: 640px) {
  .picker {
    grid-template-columns: minmax(0, 1fr);
  }
  .picker-desc {
    min-height: 0;
  }
  .item {
    flex-wrap: wrap;
  }
}
</style>

<style>
.provider-picker-modal {
  max-width: calc(100vw - 32px);
  background: var(--bg-base);
}
.provider-picker-modal .arco-modal-header {
  height: auto;
  padding: 24px 24px 8px;
  border-bottom: none;
}
.provider-picker-modal .arco-modal-title {
  font-size: 18px;
}
.provider-picker-modal .arco-modal-body {
  max-height: calc(100vh - 160px);
  padding: 16px 24px 24px;
  overflow-y: auto;
}
</style>
