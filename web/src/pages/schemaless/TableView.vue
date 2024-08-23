<template>
  <t-table
      :data="records"
      :columns="columns"
      row-key="uid"
      vertical-align="top"
      :hover="true"
      :pagination="pagination"
      :loading="isLoading"
      @page-change="pagination = $event; queryRecords()"
  >
  </t-table>
</template>

<script setup lang="ts">
import {ref, onMounted} from "vue";
import {useRoute} from "vue-router";
import {getView, queryView, View} from "@/api/schemalessView.ts";
import {Field, listFields} from "@/api/schemalessField.ts";
import type {PaginationProps, PrimaryTableCol, TableRowData} from "tdesign-vue-next";

const route = useRoute()
const projectUID = route.params.uid as string
const viewUID = route.params.viewUID as string

const isLoading = ref(false)
const pagination = ref<PaginationProps>({
  pageSize: 10,
  total: 0,
  current: 1,
})
const view = ref<View>({} as View)
const fields = ref<Field[]>([])

const columns = ref<PrimaryTableCol<TableRowData>[]>([])
const records = ref<any[]>([])
const queryRecords = () => {
  isLoading.value = true
  queryView(projectUID, viewUID, {
    page: pagination.value.current,
    pageSize: pagination.value.pageSize,
  }).then(res => {
    records.value = res.records
    pagination.value.total = res.total
  }).finally(() => {
    isLoading.value = false
  })
}

onMounted(async () => {
  isLoading.value = true

  view.value = await getView(projectUID, viewUID)
  fields.value = await listFields(projectUID, view.value.table.uid)
  columns.value = view.value.fieldUIDs.map(fieldUID => {
    const field = fields.value.find(field => field.uid === fieldUID)
    if (!field) {
      return {
        colKey: fieldUID,
        title: fieldUID,
      }
    }
    return {
      colKey: field.name,
      title: field.label,
    }
  })
  queryRecords()

  isLoading.value = false
})
</script>

<style scoped>

</style>
