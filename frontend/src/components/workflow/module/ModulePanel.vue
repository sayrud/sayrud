<script setup lang="ts">
import { computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import FieldWidget from '@/components/workflow/module/FieldWidget.vue'
import OutputList from '@/components/workflow/OutputList.vue'
import PanelSection from '@/components/workflow/PanelSection.vue'
import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import { dslText, isBlank } from '@/utils/modules'
import { contextBefore, findStep, moduleDef, stepTable } from '@/utils/workflow'

/** Renders the configuration form of a step from the sections of its module DSL. */
const props = defineProps<{ stepId: string }>()
const { t } = useI18n()
const store = useWorkflowEditorStore()
const step = computed(() => findStep(store.spec!, props.stepId)!)
const def = computed(() => moduleDef(step.value.type))
const ctx = computed(() => (store.spec ? contextBefore(store.spec, props.stepId) : null))

watch(() => stepTable(step.value), (uid) => void store.ensureFields(uid), { immediate: true })

const pluginID = computed(() => (step.value.type.startsWith('plugin:') ? step.value.type.slice('plugin:'.length) : ''))

function visible(showIf: string | undefined) {
  return !showIf || !isBlank(step.value.config[showIf])
}
</script>

<template>
  <template v-if="def">
    <a-alert v-if="def.plugin && !def.plugin.enabled" type="warning" class="plugin-alert">{{ t('workflow.inspector.pluginDisabled') }}</a-alert>
    <template v-for="(section, i) in def.sections" :key="i">
      <PanelSection v-if="visible(section.showIf)" :title="dslText(section.title)" :description="dslText(section.description) || undefined">
        <template v-if="section.extra && visible(section.extra.showIf)" #extra>
          <FieldWidget :step-id="stepId" :field="section.extra" :ctx="ctx" compact />
        </template>
        <template v-for="(field, j) in section.fields" :key="field.key ?? j">
          <FieldWidget v-if="visible(field.showIf)" :step-id="stepId" :field="field" :ctx="ctx" />
        </template>
      </PanelSection>
    </template>
    <OutputList :step-id="stepId" />
  </template>
  <PanelSection
    v-else
    :title="step.type"
    :description="pluginID ? t('workflow.inspector.pluginMissing', { plugin: pluginID }) : t('workflow.inspector.unknownModule')"
  />
</template>

<style scoped>
.plugin-alert {
  margin-bottom: 12px;
}
</style>
