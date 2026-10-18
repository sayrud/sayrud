<script setup lang="ts">
import { Puzzle, Search, SquareArrowOutUpRight } from '@lucide/vue'
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'

import { useAuthStore } from '@/stores/auth'
import type { ModuleCategory, StepType } from '@/types/workflow'
import { MODULE_CATEGORIES, moduleDef, stepCategory, stepMeta } from '@/utils/modules'

const props = defineProps<{ types: StepType[] }>()
const emit = defineEmits<{ select: [type: StepType]; close: [] }>()
const { t } = useI18n()
const auth = useAuthStore()
const router = useRouter()

/** Admins manage the installed nodes in the admin console, opened in a new tab to keep the draft. */
const manageHref = computed(() => (auth.user?.isAdmin ? router.resolve({ name: 'admin-workflow-plugins' }).href : ''))
const isPlugin = (type: StepType) => !!moduleDef(type)?.plugin

const CATEGORY_LABELS: Record<ModuleCategory, () => string> = {
  data: () => t('workflow.picker.data'),
  logic: () => t('workflow.picker.logic'),
  integration: () => t('workflow.picker.integration'),
}

const query = ref('')
const tab = ref<'all' | ModuleCategory>('all')
const active = ref(0)
const input = ref<{ focus: () => void }>()
const list = ref<HTMLElement>()

const categories = computed(() => MODULE_CATEGORIES.filter((c) => props.types.some((type) => stepCategory(type) === c)))

const keyword = computed(() => query.value.trim().toLowerCase())

// Searching covers all categories; otherwise the selected tab narrows the groups.
const groups = computed(() =>
  categories.value
    .filter((c) => keyword.value || tab.value === 'all' || tab.value === c)
    .map((c) => ({
      category: c,
      types: props.types.filter((type) => {
        if (stepCategory(type) !== c) return false
        if (!keyword.value) return true
        const meta = stepMeta(type)
        return meta.label.toLowerCase().includes(keyword.value) || meta.description.toLowerCase().includes(keyword.value)
      }),
    }))
    .filter((g) => g.types.length),
)

const flat = computed(() => groups.value.flatMap((g) => g.types))

watch([keyword, tab], () => {
  active.value = 0
  list.value?.scrollTo({ top: 0 })
})

/** Splits the label around the keyword so that the match can be highlighted. */
function parts(label: string) {
  const i = keyword.value ? label.toLowerCase().indexOf(keyword.value) : -1
  if (i < 0) return [{ text: label, hit: false }]
  const end = i + keyword.value.length
  return [
    { text: label.slice(0, i), hit: false },
    { text: label.slice(i, end), hit: true },
    { text: label.slice(end), hit: false },
  ].filter((p) => p.text)
}

function move(delta: number) {
  const n = flat.value.length
  if (!n) return
  active.value = (active.value + delta + n) % n
  void nextTick(() => list.value?.querySelector('.option.active')?.scrollIntoView({ block: 'nearest' }))
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
    e.preventDefault()
    move(e.key === 'ArrowDown' ? 1 : -1)
  } else if (e.key === 'Enter' && !e.isComposing) {
    const type = flat.value[active.value]
    if (type) emit('select', type)
  } else if (e.key === 'Escape') {
    emit('close')
  }
}

onMounted(() => void nextTick(() => input.value?.focus()))
</script>

<template>
  <div class="step-picker" @keydown="onKeydown">
    <div class="search">
      <a-input ref="input" v-model="query" :placeholder="t('workflow.picker.search')" allow-clear class="search-input">
        <template #prefix><Search :size="15" /></template>
      </a-input>
    </div>

    <a-tabs v-if="!keyword && categories.length > 1" v-model:active-key="tab" size="medium" class="tabs">
      <a-tab-pane key="all" :title="t('workflow.picker.all')" />
      <a-tab-pane v-for="c in categories" :key="c" :title="CATEGORY_LABELS[c]()" />
    </a-tabs>

    <div ref="list" class="list">
      <div v-for="g in groups" :key="g.category" class="group">
        <div class="group-title">{{ CATEGORY_LABELS[g.category]() }}</div>
        <a-tooltip v-for="type in g.types" :key="type" :content="stepMeta(type).description" position="right" :mouse-enter-delay="300">
          <div
            class="option"
            :class="{ active: flat[active] === type }"
            role="option"
            :aria-selected="flat[active] === type"
            @mouseenter="active = flat.indexOf(type)"
            @click="emit('select', type)"
          >
            <span class="option-icon" :style="{ background: stepMeta(type).color }">
              <component :is="stepMeta(type).icon" :size="13" :stroke-width="2.25" />
            </span>
            <span class="option-label">
              <template v-for="(p, i) in parts(stepMeta(type).label)" :key="i">
                <mark v-if="p.hit">{{ p.text }}</mark>
                <template v-else>{{ p.text }}</template>
              </template>
            </span>
            <a-tooltip v-if="isPlugin(type)" :content="t('workflow.picker.pluginTip')" :mouse-enter-delay="300">
              <Puzzle :size="12" class="option-plugin" />
            </a-tooltip>
          </div>
        </a-tooltip>
      </div>
      <div v-if="!flat.length" class="empty">{{ t('workflow.picker.empty') }}</div>
    </div>

    <a v-if="manageHref" :href="manageHref" target="_blank" rel="noopener" class="manage">
      <Puzzle :size="14" />
      <span>{{ t('workflow.picker.manage') }}</span>
      <SquareArrowOutUpRight :size="12" class="manage-external" />
    </a>
  </div>
</template>

<style scoped>
.step-picker {
  display: flex;
  flex-direction: column;
  width: 320px;
  max-height: 480px;
}
.search {
  padding: 8px 8px 6px;
  border-bottom: 1px solid var(--line-border);
}
.search-input,
.search-input:hover,
.search-input.arco-input-focus {
  border-color: transparent;
  background: transparent;
  box-shadow: none;
}
.search-input :deep(.arco-input-prefix) {
  color: var(--text-placeholder);
}
.tabs {
  flex: none;
}
.tabs :deep(.arco-tabs-nav) {
  padding: 0 16px;
}
.tabs :deep(.arco-tabs-nav::before) {
  background: var(--line-border);
}
.tabs :deep(.arco-tabs-tab) {
  display: inline-flex;
  align-items: center;
  height: 40px;
  margin: 0 20px 0 0;
  padding: 0;
}
.tabs :deep(.arco-tabs-tab-title) {
  padding: 0;
}
.tabs :deep(.arco-tabs-tab-title::before) {
  display: none;
}
.tabs :deep(.arco-tabs-content) {
  display: none;
}
.list {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 4px 6px 6px;
}
.group-title {
  padding: 10px 10px 4px;
  font-size: 12px;
  color: var(--text-placeholder);
}
.option {
  display: flex;
  align-items: center;
  gap: 10px;
  height: 36px;
  padding: 0 10px;
  border-radius: var(--border-radius-medium);
  color: var(--text-title);
  cursor: pointer;
}
.option.active {
  background: var(--fill-hover);
}
.option-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border-radius: var(--border-radius-small);
  color: #fff;
  flex: none;
}
.option-label {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}
.option-label mark {
  padding: 0;
  background: transparent;
  color: var(--color-primary);
}
.option-plugin {
  flex: none;
  margin-left: auto;
  color: var(--text-placeholder);
}
.manage {
  display: flex;
  flex: none;
  align-items: center;
  gap: 8px;
  height: 38px;
  padding: 0 16px;
  border-top: 1px solid var(--line-border);
  color: var(--text-caption);
  font-size: 13px;
  text-decoration: none;
  transition:
    color 0.15s,
    background-color 0.15s;
}
.manage:hover {
  background: var(--fill-hover);
  color: var(--color-primary);
}
.manage-external {
  margin-left: auto;
  opacity: 0.6;
}
.empty {
  padding: 32px 0;
  font-size: 13px;
  text-align: center;
  color: var(--text-placeholder);
}
</style>
