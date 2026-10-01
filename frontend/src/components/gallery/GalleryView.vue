<script setup lang="ts">
import { Plus } from '@lucide/vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import RecordCard from '@/components/kanban/RecordCard.vue'
import { keepRecordInView, useViewData } from '@/composables/useViewData'
import { useBaseStore } from '@/stores/base'
import type { SLView } from '@/types/bitable'

const { t } = useI18n()

const props = defineProps<{ view: SLView }>()
const store = useBaseStore()
const viewRef = computed(() => props.view)
const { visibleFields, searchedRows } = useViewData(viewRef)

const uids = computed(() => searchedRows.value.map((r) => r.uid))

async function add() {
  const r = await store.createRecord()
  if (r) {
    keepRecordInView(props.view.uid, r.uid)
    store.expandRecord(r.uid, [...uids.value, r.uid])
  }
}
</script>

<template>
  <div class="gallery">
    <div class="gallery-grid">
      <RecordCard
        v-for="r in searchedRows"
        :key="r.uid"
        :record="r"
        :fields="visibleFields"
        :max-fields="8"
        show-empty
        class="gallery-card"
        @open="store.expandRecord(r.uid, uids)"
      />
      <button v-if="store.canEdit" class="add-card" @click="add"><Plus :size="20" /> {{ t('grid.addRecord') }}</button>
    </div>
    <div v-if="!searchedRows.length && store.search.term" class="empty">{{ t('gallery.noMatch', { term: store.search.term }) }}</div>
  </div>
</template>

<style scoped>
.gallery {
  flex: 1;
  min-width: 0;
  overflow: auto;
  padding: 16px;
  background: var(--bg-base);
}
.gallery-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 16px;
  align-items: start;
}
.gallery-card {
  min-height: 160px;
}
.add-card {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-height: 160px;
  border: 1px dashed var(--line-border-strong);
  border-radius: 8px;
  background: transparent;
  color: var(--text-caption);
  cursor: pointer;
}
.add-card:hover {
  color: var(--color-primary);
  border-color: var(--color-primary);
  background: var(--bg-body);
}
.empty {
  margin-top: 40px;
  text-align: center;
  color: var(--text-placeholder);
}
</style>
