<script setup lang="ts">
import { Search, SquareFunction } from '@lucide/vue'
import { computed, nextTick, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import VarMenuItems from '@/components/workflow/VarMenuItems.vue'
import { VAR_KIND_ICONS, type RefEntry, type VarNode } from '@/utils/workflow'

/** Content of the "Add value" dropdown: formula plus referenced values grouped by trigger and step; on pick, the dropdown emits select with the option value. */
const props = defineProps<{ entries: RefEntry[]; formula?: boolean; formulaActive?: boolean }>()
const { t } = useI18n()

const keyword = ref('')
const search = ref<{ focus: () => void }>()

onMounted(() => {
  void nextTick(() => search.value?.focus())
})

/** While searching, list matching variables flat along with where they live. */
const matches = computed(() => {
  const k = keyword.value.trim().toLowerCase()
  if (!k) return []
  const out: { node: VarNode; trail: string }[] = []
  const walk = (nodes: VarNode[], trail: string) => {
    for (const n of nodes) {
      if (n.label.toLowerCase().includes(k) || n.path.toLowerCase().includes(k)) out.push({ node: n, trail })
      if (n.children) walk(n.children, `${trail} › ${n.label}`)
    }
  }
  for (const e of props.entries) walk(e.nodes, e.label)
  return out.slice(0, 50)
})
</script>

<script lang="ts">
/** Value of the "Add formula" option. */
export const FORMULA_OPTION = '__formula__'
</script>

<template>
  <div class="menu-search" @click.stop @keydown.stop>
    <a-input ref="search" v-model="keyword" size="small" allow-clear :placeholder="t('workflow.value.searchOrPick')">
      <template #prefix><Search :size="14" /></template>
    </a-input>
  </div>

  <template v-if="keyword.trim()">
    <a-doption v-for="m in matches" :key="m.node.path" :value="m.node.path">
      <template #icon><component :is="VAR_KIND_ICONS[m.node.kind]" :size="14" /></template>
      <span class="match-label">{{ m.node.label }}</span>
      <span class="match-trail">{{ m.trail }}</span>
    </a-doption>
    <a-empty v-if="!matches.length" :description="t('workflow.value.noVars')" class="menu-empty" />
  </template>
  <template v-else>
    <a-dgroup v-if="formula" :title="t('workflow.value.formulaGroup')">
      <a-doption :value="FORMULA_OPTION">
        <template #icon><SquareFunction :size="15" class="formula-icon" /></template>
        {{ formulaActive ? t('workflow.value.editFormula') : t('workflow.value.addFormula') }}
      </a-doption>
    </a-dgroup>
    <a-dgroup :title="t('workflow.value.refGroup')">
      <a-dsubmenu v-for="entry in entries" :key="entry.key" trigger="hover" :popup-max-height="360">
        <template #icon><component :is="entry.icon" :size="15" :style="{ color: entry.color }" /></template>
        {{ entry.label }}
        <template #content>
          <VarMenuItems :nodes="entry.nodes" />
        </template>
      </a-dsubmenu>
    </a-dgroup>
    <a-empty v-if="!entries.length" :description="t('workflow.value.noVars')" class="menu-empty" />
  </template>
</template>

<style scoped>
.menu-search {
  width: 280px;
  padding: 4px 4px 6px;
}
.formula-icon {
  color: #7f3bf5;
}
.match-label {
  margin-right: 8px;
}
.match-trail {
  font-size: 12px;
  color: var(--text-placeholder);
}
.menu-empty {
  padding: 12px 0;
}
</style>
