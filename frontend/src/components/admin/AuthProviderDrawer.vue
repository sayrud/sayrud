<script setup lang="ts">
import { Message, type FieldRule, type FormInstance } from '@arco-design/web-vue'
import { Copy } from '@lucide/vue'
import { computed, reactive, ref, toRaw, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { ssoApi, type AdminAuthProvider } from '@/api/sso'
import ProviderIcon from '@/components/common/ProviderIcon.vue'
import { ICONS, SLUG_PATTERN, providerURLs, type ProviderDraft } from '@/utils/ssoTemplates'

const props = defineProps<{
  /** The sign-in method to edit, null to create one. */
  provider: AdminAuthProvider | null
  /** Initial data from the template when creating. */
  initial: ProviderDraft | null
  siteUrl: string
}>()
const visible = defineModel<boolean>('visible', { required: true })
const emit = defineEmits<{ saved: [provider: AdminAuthProvider] }>()

const { t } = useI18n()

const TYPE_LABELS: Record<ProviderDraft['type'], string> = { oauth2: 'OAuth 2.0', oidc: 'OIDC', saml: 'SAML 2.0', ldap: 'LDAP' }

const formRef = ref<FormInstance>()
const draft = reactive<ProviderDraft>(emptyDraft())
const metadataSource = ref<'url' | 'xml'>('url')
const testUsername = ref('')
const saving = ref(false)
const testing = ref(false)

function emptyDraft(): ProviderDraft {
  return {
    name: '',
    slug: '',
    icon: 'oidc',
    type: 'oidc',
    enabled: true,
    config: {},
    clientSecret: '',
    bindPassword: '',
    autoCreateUser: false,
    linkByEmail: true,
    allowedEmailDomains: [],
    allowedGroups: [],
  }
}

watch(visible, (open) => {
  if (!open) return
  const p = props.provider
  const next: ProviderDraft = p
    ? {
        name: p.name,
        slug: p.slug,
        icon: p.icon,
        type: p.type,
        enabled: p.enabled,
        config: structuredClone(toRaw(p.config)),
        clientSecret: '',
        bindPassword: '',
        autoCreateUser: p.autoCreateUser,
        linkByEmail: p.linkByEmail,
        allowedEmailDomains: [...p.allowedEmailDomains],
        allowedGroups: [...p.allowedGroups],
      }
    : structuredClone(toRaw(props.initial) ?? emptyDraft())
  Object.assign(draft, next)
  draft.config.scopes ??= []
  metadataSource.value = draft.config.idpMetadataXML ? 'xml' : 'url'
  testUsername.value = ''
  formRef.value?.clearValidate()
})

const isEdit = computed(() => !!props.provider)
const title = computed(() =>
  isEdit.value
    ? t('sso.admin.editTitle', { name: props.provider!.name })
    : t('sso.admin.createTitle', { name: props.initial?.name ?? TYPE_LABELS[draft.type] }),
)
const urls = computed(() => providerURLs(props.siteUrl, draft.slug, draft.type))
const isRedirect = computed(() => draft.type !== 'ldap')
const canTest = computed(() => draft.type !== 'oauth2')
const hasClientSecret = computed(() => !!props.provider?.hasClientSecret)
const hasBindPassword = computed(() => !!props.provider?.hasBindPassword)

const required = (field: string): FieldRule[] => [{ required: true, message: t('sso.admin.required', { field }) }]
const rules = computed<Record<string, FieldRule[]>>(() => ({
  name: required(t('sso.admin.name')),
  slug: [
    ...required(t('sso.admin.slug')),
    { validator: (v, cb) => cb(SLUG_PATTERN.test(v ?? '') ? undefined : t('sso.admin.slugInvalid')) },
  ],
  'config.authURL': required(t('sso.admin.authURL')),
  'config.tokenURL': required(t('sso.admin.tokenURL')),
  'config.userInfoURL': required(t('sso.admin.userInfoURL')),
  'config.clientID': required('Client ID'),
  clientSecret: hasClientSecret.value ? [] : required('Client Secret'),
  'config.issuer': required('Issuer'),
  'config.idpMetadataURL': required(t('sso.admin.metadataURL')),
  'config.idpMetadataXML': required(t('sso.admin.metadataXML')),
  'config.url': required(t('sso.admin.ldapURL')),
  'config.baseDN': required('Base DN'),
  'config.userFilter': [
    ...required(t('sso.admin.userFilter')),
    {
      validator: (v, cb) =>
        cb((v ?? '').includes('{username}') ? undefined : t('sso.admin.userFilterInvalid', { placeholder: '{username}' })),
    },
  ],
}))

/** Only submits the field of the selected metadata source and clears the other. */
function payloadConfig() {
  const config = { ...draft.config }
  if (draft.type === 'saml') {
    if (metadataSource.value === 'url') config.idpMetadataXML = ''
    else config.idpMetadataURL = ''
  }
  return config
}

async function save() {
  saving.value = true
  try {
    const body = {
      name: draft.name.trim(),
      icon: draft.icon,
      enabled: draft.enabled,
      config: payloadConfig(),
      clientSecret: draft.clientSecret || undefined,
      bindPassword: draft.bindPassword || undefined,
      autoCreateUser: draft.autoCreateUser,
      linkByEmail: draft.linkByEmail,
      allowedEmailDomains: draft.allowedEmailDomains,
      allowedGroups: draft.allowedGroups,
    }
    const saved = props.provider
      ? await ssoApi.update(props.provider.id, body)
      : await ssoApi.create({ ...body, slug: draft.slug.trim(), type: draft.type })
    Message.success(t('common.saved'))
    emit('saved', saved)
    visible.value = false
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    saving.value = false
  }
}

async function submit() {
  if (await formRef.value?.validate()) return
  await save()
}

async function test() {
  testing.value = true
  try {
    await ssoApi.test({
      id: props.provider?.id,
      type: draft.type,
      config: payloadConfig(),
      bindPassword: draft.bindPassword || undefined,
      username: testUsername.value.trim() || undefined,
    })
    Message.success(t('sso.admin.testSuccess'))
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    testing.value = false
  }
}

async function copy(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    Message.success(t('sso.admin.copied'))
  } catch {
    Message.error(t('sso.admin.copyFailed'))
  }
}
</script>

<template>
  <a-drawer v-model:visible="visible" :width="560" :title="title" unmount-on-close :footer="true">
    <a-form ref="formRef" :model="draft" :rules="rules" layout="vertical">
      <div class="section-title">{{ t('sso.admin.basic') }}</div>
      <div class="grid">
        <a-form-item field="name" :label="t('sso.admin.name')" :extra="t('sso.admin.nameDescription')">
          <a-input v-model="draft.name" :max-length="32" />
        </a-form-item>
        <a-form-item field="slug" :label="t('sso.admin.slug')" :extra="t('sso.admin.slugDescription')">
          <a-input v-model="draft.slug" :max-length="32" :disabled="isEdit" />
        </a-form-item>
      </div>
      <div class="grid">
        <a-form-item :label="t('sso.admin.icon')">
          <a-select v-model="draft.icon">
            <a-option v-for="icon in ICONS" :key="icon" :value="icon">
              <span class="icon-option"><ProviderIcon :icon="icon" :size="16" />{{ icon }}</span>
            </a-option>
          </a-select>
        </a-form-item>
        <a-form-item :label="t('sso.admin.enabled')">
          <a-switch v-model="draft.enabled" />
        </a-form-item>
      </div>

      <template v-if="isRedirect">
        <div class="section-title">{{ t('sso.admin.idpInfo') }}</div>
        <a-alert v-if="!siteUrl" type="warning" class="block">{{ t('sso.admin.urlNeedsExternal') }}</a-alert>
        <template v-else>
          <div v-if="urls.callbackURL" class="readonly">
            <div class="readonly-label">{{ t('sso.admin.callbackURL') }}</div>
            <a-input :model-value="urls.callbackURL" readonly>
              <template #suffix>
                <a-button size="mini" type="text" :aria-label="t('sso.admin.copy')" @click="copy(urls.callbackURL)"><Copy :size="14" /></a-button>
              </template>
            </a-input>
          </div>
          <template v-if="urls.acsURL">
            <div class="readonly">
              <div class="readonly-label">{{ t('sso.admin.acsURL') }}</div>
              <a-input :model-value="urls.acsURL" readonly>
                <template #suffix>
                  <a-button size="mini" type="text" :aria-label="t('sso.admin.copy')" @click="copy(urls.acsURL)"><Copy :size="14" /></a-button>
                </template>
              </a-input>
            </div>
            <div class="readonly">
              <div class="readonly-label">{{ t('sso.admin.entityID') }}</div>
              <a-input :model-value="urls.metadataURL" readonly>
                <template #suffix>
                  <a-button size="mini" type="text" :aria-label="t('sso.admin.copy')" @click="copy(urls.metadataURL)"><Copy :size="14" /></a-button>
                </template>
              </a-input>
              <div class="text-desc hint">{{ isEdit ? t('sso.admin.metadataHint') : t('sso.admin.samlCertHint') }}</div>
            </div>
          </template>
        </template>
      </template>

      <div class="section-title">{{ t('sso.admin.connection') }}</div>

      <template v-if="draft.type === 'oauth2'">
        <a-form-item field="config.authURL" :label="t('sso.admin.authURL')">
          <a-input v-model="draft.config.authURL" placeholder="https://" />
        </a-form-item>
        <a-form-item field="config.tokenURL" :label="t('sso.admin.tokenURL')">
          <a-input v-model="draft.config.tokenURL" placeholder="https://" />
        </a-form-item>
        <a-form-item field="config.userInfoURL" :label="t('sso.admin.userInfoURL')">
          <a-input v-model="draft.config.userInfoURL" placeholder="https://" />
        </a-form-item>
        <a-form-item field="config.emailsURL" :label="t('sso.admin.emailsURL')" :extra="t('sso.admin.emailsURLDescription')">
          <a-input v-model="draft.config.emailsURL" placeholder="https://" />
        </a-form-item>
      </template>

      <a-form-item v-if="draft.type === 'oidc'" field="config.issuer" label="Issuer" :extra="t('sso.admin.issuerDescription')">
        <a-input v-model="draft.config.issuer" placeholder="https://accounts.google.com" />
      </a-form-item>

      <template v-if="draft.type === 'oauth2' || draft.type === 'oidc'">
        <div class="grid">
          <a-form-item field="config.clientID" label="Client ID">
            <a-input v-model="draft.config.clientID" />
          </a-form-item>
          <a-form-item field="clientSecret" label="Client Secret">
            <a-input-password v-model="draft.clientSecret" :placeholder="hasClientSecret ? t('sso.admin.secretKeep') : ''" autocomplete="new-password" />
          </a-form-item>
        </div>
        <a-form-item label="Scopes">
          <a-input-tag v-model="draft.config.scopes" :placeholder="t('sso.admin.tagPlaceholder')" allow-clear />
        </a-form-item>
      </template>

      <template v-if="draft.type === 'oauth2'">
        <div class="sub-title">{{ t('sso.admin.fieldMapping') }}<span class="text-desc">{{ t('sso.admin.pathHint') }}</span></div>
        <div class="grid">
          <a-form-item :label="t('sso.admin.subjectPath')">
            <a-input v-model="draft.config.subjectPath" placeholder="id" />
          </a-form-item>
          <a-form-item :label="t('sso.admin.emailPath')">
            <a-input v-model="draft.config.emailPath" placeholder="email" />
          </a-form-item>
          <a-form-item :label="t('sso.admin.namePath')">
            <a-input v-model="draft.config.namePath" placeholder="name" />
          </a-form-item>
          <a-form-item :label="t('sso.admin.groupsPath')">
            <a-input v-model="draft.config.groupsPath" />
          </a-form-item>
        </div>
      </template>

      <div v-if="draft.type === 'oidc'" class="grid">
        <a-form-item :label="t('sso.admin.nameClaim')">
          <a-input v-model="draft.config.nameClaim" placeholder="name" />
        </a-form-item>
        <a-form-item :label="t('sso.admin.groupsClaim')">
          <a-input v-model="draft.config.groupsClaim" placeholder="groups" />
        </a-form-item>
      </div>

      <template v-if="draft.type === 'saml'">
        <a-form-item :label="t('sso.admin.metadataSource')">
          <a-radio-group v-model="metadataSource" type="button">
            <a-radio value="url">{{ t('sso.admin.metadataURL') }}</a-radio>
            <a-radio value="xml">{{ t('sso.admin.metadataXML') }}</a-radio>
          </a-radio-group>
        </a-form-item>
        <a-form-item v-if="metadataSource === 'url'" field="config.idpMetadataURL" :label="t('sso.admin.metadataURL')">
          <a-input v-model="draft.config.idpMetadataURL" placeholder="https://" />
        </a-form-item>
        <a-form-item v-else field="config.idpMetadataXML" :label="t('sso.admin.metadataXML')">
          <a-textarea v-model="draft.config.idpMetadataXML" :auto-size="{ minRows: 4, maxRows: 10 }" class="mono" />
        </a-form-item>
        <a-alert type="info" class="block">{{ t('sso.admin.nameIDHint') }}</a-alert>
      </template>

      <template v-if="draft.type === 'ldap'">
        <div class="grid">
          <a-form-item field="config.url" :label="t('sso.admin.ldapURL')" :extra="t('sso.admin.ldapURLDescription')">
            <a-input v-model="draft.config.url" placeholder="ldaps://ldap.example.com" />
          </a-form-item>
          <a-form-item label="StartTLS" :extra="t('sso.admin.startTLSDescription')">
            <a-switch v-model="draft.config.startTLS" :disabled="!draft.config.url?.startsWith('ldap://')" />
          </a-form-item>
        </div>
        <div class="grid">
          <a-form-item label="Bind DN" :extra="t('sso.admin.bindDNDescription')">
            <a-input v-model="draft.config.bindDN" placeholder="cn=sayrud,ou=services,dc=example,dc=com" />
          </a-form-item>
          <a-form-item :label="t('sso.admin.bindPassword')">
            <a-input-password v-model="draft.bindPassword" :placeholder="hasBindPassword ? t('sso.admin.secretKeep') : ''" autocomplete="new-password" />
          </a-form-item>
        </div>
        <a-form-item field="config.baseDN" label="Base DN">
          <a-input v-model="draft.config.baseDN" placeholder="dc=example,dc=com" />
        </a-form-item>
        <a-form-item field="config.userFilter" :label="t('sso.admin.userFilter')" :extra="t('sso.admin.userFilterDescription', { placeholder: '{username}' })">
          <a-input v-model="draft.config.userFilter" class="mono" />
        </a-form-item>
        <a-form-item :label="t('sso.admin.rootCA')" :extra="t('sso.admin.rootCADescription')">
          <a-textarea v-model="draft.config.rootCA" :auto-size="{ minRows: 2, maxRows: 8 }" class="mono" placeholder="-----BEGIN CERTIFICATE-----" />
        </a-form-item>
      </template>

      <template v-if="draft.type === 'saml' || draft.type === 'ldap'">
        <div class="sub-title">
          {{ t('sso.admin.attributeMapping') }}
          <span v-if="draft.type === 'saml'" class="text-desc">{{ t('sso.admin.autoDetect') }}</span>
        </div>
        <div class="grid">
          <a-form-item v-if="draft.type === 'ldap'" :label="t('sso.admin.subjectAttribute')">
            <a-input v-model="draft.config.subjectAttribute" placeholder="uid" />
          </a-form-item>
          <a-form-item :label="t('sso.admin.emailAttribute')">
            <a-input v-model="draft.config.emailAttribute" :placeholder="draft.type === 'ldap' ? 'mail' : ''" />
          </a-form-item>
          <a-form-item :label="t('sso.admin.nameAttribute')">
            <a-input v-model="draft.config.nameAttribute" :placeholder="draft.type === 'ldap' ? 'displayName' : ''" />
          </a-form-item>
          <a-form-item :label="t('sso.admin.groupsAttribute')">
            <a-input v-model="draft.config.groupsAttribute" :placeholder="draft.type === 'ldap' ? 'memberOf' : ''" />
          </a-form-item>
        </div>
      </template>

      <a-form-item v-if="draft.type === 'saml'" :label="t('sso.admin.allowIdPInitiated')" :extra="t('sso.admin.allowIdPInitiatedDescription')">
        <a-switch v-model="draft.config.allowIdPInitiated" />
      </a-form-item>
      <a-form-item v-if="draft.type !== 'oidc'" :label="t('sso.admin.trustEmail')" :extra="t('sso.admin.trustEmailDescription')">
        <a-switch v-model="draft.config.trustEmail" />
      </a-form-item>

      <div class="section-title">{{ t('sso.admin.account') }}</div>
      <a-form-item :label="t('sso.admin.autoCreateUser')" :extra="t('sso.admin.autoCreateUserDescription')">
        <a-switch v-model="draft.autoCreateUser" />
      </a-form-item>
      <a-form-item :label="t('sso.admin.linkByEmail')" :extra="t('sso.admin.linkByEmailDescription')">
        <a-switch v-model="draft.linkByEmail" />
      </a-form-item>
      <a-form-item :label="t('sso.admin.allowedEmailDomains')" :extra="t('sso.admin.allowedEmailDomainsDescription')">
        <a-input-tag v-model="draft.allowedEmailDomains" :placeholder="t('sso.admin.tagPlaceholder')" allow-clear />
      </a-form-item>
      <a-form-item :label="t('sso.admin.allowedGroups')" :extra="t('sso.admin.allowedGroupsDescription')">
        <a-input-tag v-model="draft.allowedGroups" :placeholder="t('sso.admin.tagPlaceholder')" allow-clear />
      </a-form-item>
    </a-form>

    <template #footer>
      <div class="footer">
        <a-space v-if="canTest">
          <a-input v-if="draft.type === 'ldap'" v-model="testUsername" :placeholder="t('sso.admin.testUsername')" size="small" class="test-user" />
          <a-button :loading="testing" @click="test">{{ t('sso.admin.test') }}</a-button>
        </a-space>
        <a-space class="footer-actions">
          <a-button @click="visible = false">{{ t('common.cancel') }}</a-button>
          <a-button type="primary" :loading="saving" @click="submit">{{ t('common.save') }}</a-button>
        </a-space>
      </div>
    </template>
  </a-drawer>
</template>

<style scoped>
.section-title {
  margin: 8px 0 12px;
  font-size: 14px;
  font-weight: 600;
  color: var(--text-title);
}
.section-title:not(:first-child) {
  margin-top: 24px;
  padding-top: 20px;
  border-top: 1px solid var(--line-border);
}
.sub-title {
  display: flex;
  gap: 8px;
  align-items: baseline;
  margin: 4px 0 12px;
  font-weight: 500;
  color: var(--text-title);
}
.grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0 16px;
}
.icon-option {
  display: inline-flex;
  gap: 8px;
  align-items: center;
}
.block {
  margin-bottom: 16px;
}
.readonly {
  margin-bottom: 16px;
}
.readonly-label {
  margin-bottom: 8px;
  color: var(--text-body);
}
.hint {
  margin-top: 4px;
}
.mono :deep(textarea),
.mono :deep(input) {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
}
.footer {
  display: flex;
  align-items: center;
}
.footer-actions {
  margin-left: auto;
}
.test-user {
  width: 160px;
}
@media (max-width: 640px) {
  .grid {
    grid-template-columns: 1fr;
  }
}
</style>
