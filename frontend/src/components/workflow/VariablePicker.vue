<script setup lang="ts">
import { Search } from '@lucide/vue'
import type { TreeNodeData } from '@arco-design/web-vue'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { VAR_KIND_ICONS, type VarGroup, type VarNode } from '@/utils/workflow'

const props = defineProps<{ groups: VarGroup[] }>()
const emit = defineEmits<{ select: [path: string] }>()
const { t } = useI18n()

const keyword = ref('')
const GROUP_PREFIX = 'group:'

function matches(node: VarNode, k: string): boolean {
  return node.label.toLowerCase().includes(k) || node.path.toLowerCase().includes(k) || !!node.children?.some((c) => matches(c, k))
}

function toTree(nodes: VarNode[], k: string): TreeNodeData[] {
  return nodes
    .filter((n) => !k || matches(n, k))
    .map((n) => ({ key: n.path, title: n.label, path: n.path, kind: n.kind, children: n.children?.length ? toTree(n.children, k) : undefined }))
}

const data = computed<TreeNodeData[]>(() => {
  const k = keyword.value.trim().toLowerCase()
  return props.groups
    .map((g) => ({ key: GROUP_PREFIX + g.key, title: g.label, selectable: false, group: true, children: toTree(g.nodes, k) }))
    .filter((g) => g.children.length)
})

/** Groups are expanded by default; everything is expanded while searching. */
const expanded = ref<(string | number)[]>(props.groups.map((g) => GROUP_PREFIX + g.key))
const expandedKeys = computed({
  get: () => (keyword.value.trim() ? allKeys(data.value) : expanded.value),
  set: (v) => (expanded.value = v),
})
function allKeys(nodes: TreeNodeData[]): (string | number)[] {
  return nodes.flatMap((n) => [n.key!, ...allKeys(n.children ?? [])])
}

function onSelect(_keys: (string | number)[], e: { node?: TreeNodeData }) {
  const key = String(e.node?.key ?? '')
  if (key && !key.startsWith(GROUP_PREFIX)) emit('select', key)
}
</script>

<template>
  <div class="var-picker">
    <div class="picker-search">
      <a-input v-model="keyword" :placeholder="t('workflow.value.search')" allow-clear size="small">
        <template #prefix><Search :size="14" /></template>
      </a-input>
    </div>
    <a-tree
      v-if="data.length"
      v-model:expanded-keys="expandedKeys"
      class="picker-tree"
      :data="data"
      :selected-keys="[]"
      block-node
      size="small"
      @select="onSelect"
    >
      <template #title="node">
        <span v-if="node.group" class="group-title">{{ node.title }}</span>
        <span v-else class="var-row">
          <component :is="VAR_KIND_ICONS[node.kind as VarNode['kind']]" :size="13" class="kind-icon" />
          <span class="var-label ellipsis">{{ node.title }}</span>
          <span class="var-path ellipsis">{{ node.path }}</span>
        </span>
      </template>
    </a-tree>
    <a-empty v-else :description="t('workflow.value.noVars')" class="picker-empty" />
  </div>
</template>

<style scoped>
.var-picker {
  display: flex;
  flex-direction: column;
  min-height: 0;
}
.picker-search {
  padding: 8px;
  border-bottom: 1px solid var(--line-border);
}
.picker-tree {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 4px;
}
.group-title {
  font-size: 12px;
  color: var(--text-placeholder);
}
.var-row {
  display: flex;
  width: 100%;
  align-items: center;
  gap: 6px;
  min-width: 0;
}
.kind-icon {
  flex: none;
  color: var(--text-caption);
}
.var-label {
  flex: 0 1 auto;
  min-width: 0;
}
.var-path {
  flex: 1;
  min-width: 0;
  text-align: right;
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 11.5px;
  color: var(--text-placeholder);
}
.picker-tree :deep(.arco-tree-node-title) {
  flex: 1;
  min-width: 0;
}
.picker-tree :deep(.arco-tree-node-title-text) {
  flex: 1;
  display: block;
  min-width: 0;
}
.picker-empty {
  padding: 24px 0;
}
</style>
