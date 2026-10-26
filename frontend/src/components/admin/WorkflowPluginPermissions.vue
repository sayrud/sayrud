<script setup lang="ts">
import { CircleCheck, Globe, KeyRound, Route, Timer, TriangleAlert, Zap } from '@lucide/vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import type { CredentialTypeDef, FlowKind, ModuleDef } from '@/types/workflow'

/** What a node plugin can reach, shown before installing and in its details. */
const props = defineProps<{ module: ModuleDef; domains: string[]; timeout?: number; credential?: CredentialTypeDef }>()
const { t } = useI18n()

const FLOW_LABELS: Record<FlowKind, () => string> = {
  sync: () => t('pluginAdmin.flowSync'),
  async: () => t('pluginAdmin.flowAsync'),
  background: () => t('pluginAdmin.flowBackground'),
}

const anyHost = computed(() => props.domains.includes('*'))
const hosts = computed(() => props.domains.filter((d) => d !== '*'))
</script>

<template>
  <ul class="permissions">
    <li>
      <span class="perm-icon" :class="domains.length ? (anyHost ? 'warn' : 'info') : 'ok'"><Globe :size="15" /></span>
      <div class="perm-body">
        <div class="perm-title">{{ t('pluginAdmin.network') }}</div>
        <div v-if="!domains.length" class="text-desc">{{ t('pluginAdmin.networkNone') }}</div>
        <template v-else>
          <div class="text-desc">{{ anyHost ? t('pluginAdmin.networkAny') : t('pluginAdmin.networkDomains') }}</div>
          <div v-if="hosts.length" class="perm-tags">
            <a-tag v-for="d in hosts" :key="d" size="small" class="mono">{{ d }}</a-tag>
          </div>
        </template>
      </div>
    </li>
    <li>
      <span class="perm-icon" :class="module.action ? 'warn' : 'ok'"><component :is="module.action ? Zap : CircleCheck" :size="15" /></span>
      <div class="perm-body">
        <div class="perm-title">{{ t('pluginAdmin.sideEffects') }}</div>
        <div class="text-desc">{{ module.action ? t('pluginAdmin.sideEffectsYes') : t('pluginAdmin.sideEffectsNo') }}</div>
      </div>
    </li>
    <li v-if="credential">
      <span class="perm-icon info"><KeyRound :size="15" /></span>
      <div class="perm-body">
        <div class="perm-title">{{ t('pluginAdmin.credential') }}</div>
        <div class="text-desc">{{ t('pluginAdmin.credentialDefined', { name: credential.label }) }}</div>
      </div>
    </li>
    <li>
      <span class="perm-icon info"><Route :size="15" /></span>
      <div class="perm-body">
        <div class="perm-title">{{ t('pluginAdmin.flows') }}</div>
        <div class="perm-tags">
          <a-tag v-for="f in module.flows" :key="f" size="small">{{ FLOW_LABELS[f]() }}</a-tag>
        </div>
      </div>
    </li>
    <li v-if="timeout">
      <span class="perm-icon info"><Timer :size="15" /></span>
      <div class="perm-body">
        <div class="perm-title">{{ t('pluginAdmin.timeout') }}</div>
        <div class="text-desc">{{ t('pluginAdmin.timeoutValue', { n: timeout }) }}</div>
      </div>
    </li>
    <li v-if="anyHost" class="perm-warning">
      <TriangleAlert :size="14" />
      <span>{{ t('pluginAdmin.networkAnyWarning') }}</span>
    </li>
  </ul>
</template>

<style scoped>
.permissions {
  display: flex;
  flex-direction: column;
  gap: 14px;
  margin: 0;
  padding: 0;
  list-style: none;
}
.permissions > li {
  display: flex;
  gap: 12px;
}
.perm-icon {
  display: inline-flex;
  flex: none;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 8px;
}
.perm-icon.ok {
  background: rgba(var(--success-6), 0.12);
  color: var(--color-success);
}
.perm-icon.info {
  background: var(--bg-primary-soft);
  color: var(--color-primary);
}
.perm-icon.warn {
  background: rgba(var(--warning-6), 0.14);
  color: var(--color-warning);
}
.perm-body {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
  font-size: 13px;
}
.perm-title {
  font-weight: 500;
  line-height: 1;
  color: var(--text-title);
}
.perm-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 4px;
}
.mono {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
}
.permissions > li.perm-warning {
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-radius: 6px;
  background: rgba(var(--warning-6), 0.1);
  color: var(--color-warning);
  font-size: 12.5px;
}
</style>
