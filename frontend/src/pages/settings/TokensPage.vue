<script setup lang="ts">
import { Plus } from '@lucide/vue'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { accountApi } from '@/api/account'
import PageHeader from '@/components/console/PageHeader.vue'
import SettingsSection from '@/components/console/SettingsSection.vue'
import AccessTokenManager from '@/components/tokens/AccessTokenManager.vue'
import { useMemberProjectSearch } from '@/composables/useProjectSearch'

const { t } = useI18n()

const manager = ref<InstanceType<typeof AccessTokenManager>>()
const searchProjects = useMemberProjectSearch()
</script>

<template>
  <div>
    <PageHeader :title="t('accessToken.title')" :description="t('accessToken.description')">
      <template #extra>
        <a-button type="primary" @click="manager?.openCreate()">
          <template #icon><Plus :size="16" /></template>
          {{ t('accessToken.create') }}
        </a-button>
      </template>
    </PageHeader>

    <SettingsSection flush>
      <AccessTokenManager ref="manager" :api="accountApi.tokens" :search-projects="searchProjects" />
    </SettingsSection>
  </div>
</template>
