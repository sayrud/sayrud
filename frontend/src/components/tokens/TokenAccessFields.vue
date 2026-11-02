<script setup lang="ts">
import { Message, type FieldRule } from '@arco-design/web-vue'
import { TriangleAlert } from '@lucide/vue'
import dayjs from 'dayjs'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import type { AccessTokenScope } from '@/api/account'
import AppearanceIcon from '@/components/common/AppearanceIcon.vue'
import {
  ACCESS_TOKEN_SCOPES,
  TOKEN_EXPIRY_PRESETS,
  draftExpiresAt,
  type TokenAccessDraft,
  type TokenExpiry,
  type TokenProjectOption,
} from '@/utils/accessToken'

const { t } = useI18n()

/** The form items of the access of a token, placed in the a-form whose model holds the draft fields. */
const model = defineModel<TokenAccessDraft>({ required: true })
const props = defineProps<{
  searchProjects: (keyword: string) => Promise<TokenProjectOption[]>
  /** The projects of the saved token, so their labels show before searching. */
  initialProjects?: TokenProjectOption[]
}>()
const emit = defineEmits<{ restrictedChange: [] }>()

const scopeOptions = computed(() =>
  ACCESS_TOKEN_SCOPES.map((value) => ({
    value,
    label: { read: t('accessToken.scopeRead'), write: t('accessToken.scopeWrite'), events: t('accessToken.scopeEvents') }[value],
    description: {
      read: t('accessToken.scopeReadDesc'),
      write: t('accessToken.scopeWriteDesc'),
      events: t('accessToken.scopeEventsDesc'),
    }[value],
  })),
)
const expiryOptions = computed(() => [
  ...TOKEN_EXPIRY_PRESETS.map((days) => ({
    value: days as TokenExpiry,
    label: days === 0 ? t('accessToken.expiryNever') : days === 365 ? t('accessToken.expiryYear') : t('accessToken.expiryDays', { n: days }, days),
  })),
  { value: 'custom' as TokenExpiry, label: t('accessToken.expiryCustom') },
])
const expiresAt = computed(() => draftExpiresAt(model.value))

const scopeRules = computed<FieldRule[]>(() => [
  { required: true, type: 'array', message: t('accessToken.scopesRequired') },
  { validator: (v: AccessTokenScope[] | undefined, cb) => cb(v?.length ? undefined : t('accessToken.scopesRequired')) },
])
const projectRules = computed<FieldRule[]>(() => [
  { validator: (v: string[] | undefined, cb) => cb(model.value.restricted && !v?.length ? t('accessToken.projectsRequired') : undefined) },
])
const dateRules = computed<FieldRule[]>(() => [
  { validator: (v: string | undefined, cb) => cb(model.value.expiry === 'custom' && !v ? t('accessToken.expiryDateRequired') : undefined) },
])

const projectOptions = ref<TokenProjectOption[]>([...(props.initialProjects ?? [])])
const searching = ref(false)
let searchTimer: ReturnType<typeof setTimeout> | undefined
let searchSeq = 0

async function search(keyword: string) {
  const seq = ++searchSeq
  searching.value = true
  try {
    const found = await props.searchProjects(keyword.trim())
    if (seq !== searchSeq) return
    // Keep the chosen projects as options, so their labels stay after a new search.
    const chosen = projectOptions.value.filter((p) => model.value.projects.includes(p.uid) && !found.some((f) => f.uid === p.uid))
    projectOptions.value = [...chosen, ...found]
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    if (seq === searchSeq) searching.value = false
  }
}

function onSearch(keyword: string) {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => search(keyword), 300)
}
onMounted(() => search(''))
onBeforeUnmount(() => clearTimeout(searchTimer))

watch(
  () => props.initialProjects,
  (v) => {
    const extra = (v ?? []).filter((p) => !projectOptions.value.some((o) => o.uid === p.uid))
    projectOptions.value = [...extra, ...projectOptions.value]
  },
)
watch(
  () => model.value.restricted,
  () => emit('restrictedChange'),
)

function disabledDate(date?: Date) {
  return !!date && dayjs(date).isBefore(dayjs().add(1, 'day'), 'day')
}
</script>

<template>
  <a-form-item field="scopes" :label="t('accessToken.scopes')" :rules="scopeRules">
    <a-checkbox-group v-model="model.scopes" direction="vertical" class="scopes">
      <a-checkbox v-for="o in scopeOptions" :key="o.value" :value="o.value" class="scope">
        <div class="scope-label">{{ o.label }}</div>
        <div class="text-desc">{{ o.description }}</div>
      </a-checkbox>
    </a-checkbox-group>
  </a-form-item>

  <a-form-item field="projects" :label="t('accessToken.projects')" :rules="projectRules">
    <div class="block">
      <a-radio-group v-model="model.restricted" type="button">
        <a-radio :value="false">{{ t('accessToken.projectsAll') }}</a-radio>
        <a-radio :value="true">{{ t('accessToken.projectsSelected') }}</a-radio>
      </a-radio-group>
      <a-select
        v-if="model.restricted"
        v-model="model.projects"
        multiple
        allow-search
        allow-clear
        :filter-option="false"
        :loading="searching"
        :placeholder="t('accessToken.projectsPlaceholder')"
        class="projects-select"
        @search="onSearch"
      >
        <a-option v-for="p in projectOptions" :key="p.uid" :value="p.uid" :label="p.name">
          <div class="project-option">
            <AppearanceIcon :icon="p.icon" :color="p.color" :size="18" fallback="project" />
            <span class="ellipsis">{{ p.name }}</span>
          </div>
        </a-option>
      </a-select>
      <div class="text-desc">{{ model.restricted ? t('accessToken.projectsHint') : t('accessToken.projectsAllHint') }}</div>
    </div>
  </a-form-item>

  <a-form-item field="customDate" :label="t('accessToken.expiry')" :rules="dateRules">
    <div class="block">
      <a-space wrap>
        <a-select
          v-model="model.expiry"
          :options="expiryOptions"
          :trigger-props="{ contentClass: 'token-expiry-dropdown' }"
          class="expiry-select"
        />
        <a-date-picker
          v-if="model.expiry === 'custom'"
          v-model="model.customDate"
          :disabled-date="disabledDate"
          :placeholder="t('accessToken.expiryDatePlaceholder')"
          value-format="YYYY-MM-DD"
        />
      </a-space>
      <div v-if="expiresAt" class="text-desc">{{ t('accessToken.expiresOn', { date: dayjs(expiresAt).format('YYYY-MM-DD HH:mm') }) }}</div>
      <div v-else-if="model.expiry === 0" class="never-warning">
        <TriangleAlert :size="14" />
        <span>{{ t('accessToken.neverWarning') }}</span>
      </div>
    </div>
  </a-form-item>
</template>

<style scoped>
.scopes {
  gap: 10px;
}
.scope {
  align-items: flex-start;
}
.scope :deep(.arco-checkbox-icon-hover),
.scope :deep(.arco-checkbox-icon) {
  margin-top: 3px;
}
.scope-label {
  color: var(--text-title);
  line-height: 22px;
}
.block {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 8px;
  width: 100%;
}
.projects-select {
  width: 100%;
}
.project-option {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.expiry-select {
  width: 220px;
}
.never-warning {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: rgb(var(--warning-6));
}
</style>

<style>
/* The few expiration choices fit without the default 200px scroll limit. */
.token-expiry-dropdown .arco-select-dropdown-list-wrapper {
  max-height: none;
}
</style>
