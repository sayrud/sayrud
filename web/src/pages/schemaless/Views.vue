<template>
  <t-row justify="space-between">
    <div class="operation-container">
      <t-button @click="onCreateView">
        <template #icon>
          <add-icon/>
        </template>
        新建视图
      </t-button>
    </div>
  </t-row>

  <t-table
      :data="views"
      :columns="COLUMNS"
      row-key="uid"
      vertical-align="top"
      :hover="true"
      :pagination="pagination"
      :loading="isLoading"
      @page-change="pagination = $event; getViews()"
  >
    <template #path="{row}">
      <code> {{ row.path }} </code>
    </template>
    <template #createdAt="{row}">
      {{ dayjs(row.createdAt).format('YYYY-MM-DD HH:mm:ss') }}
    </template>
    <template #ops="{row}">
      <t-space>
        <t-link theme="primary" @click="onView(row)">查看</t-link>
        <t-link theme="primary" @click="onEditView(row)">编辑</t-link>
        <t-popconfirm theme="danger" content="你确定要删除该视图吗？"
                      @confirm="onDeleteView(row)">
          <t-link theme="danger" hover="color"> 删除</t-link>
        </t-popconfirm>
      </t-space>
    </template>
  </t-table>
</template>

<script setup lang="ts">
import {onMounted, ref} from "vue";
import {useRoute, useRouter} from "vue-router";
import {
  MessagePlugin,
  type PaginationProps,
  PrimaryTableCol,
  TableRowData
} from 'tdesign-vue-next';
import dayjs from 'dayjs'
import {AddIcon} from 'tdesign-icons-vue-next';
import {listViews, View, deleteView} from "@/api/schemalessView.ts";

const route = useRoute()
const router = useRouter()

const projectUID = route.params.uid as string
const COLUMNS: PrimaryTableCol<TableRowData>[] = [
  {colKey: 'name', title: '视图名称'},
  {colKey: 'createdAt', title: '创建时间'},
  {colKey: 'ops', title: '操作'},
]
const isLoading = ref(false)
const pagination = ref<PaginationProps>({
  pageSize: 10,
  total: 0,
  current: 1,
})
const views = ref<View[]>([])
const getViews = () => {
  isLoading.value = true

  listViews(projectUID, {
    page: pagination.value.current,
    pageSize: pagination.value.pageSize,
  }).then(res => {
    views.value = res.views
    pagination.value.total = res.total
  }).finally(() => {
    isLoading.value = false
  })
}

const onCreateView = () => {
  router.push({name: 'SchemalessViewCreate', params: {uid: projectUID}})
}

const onView = (view: View) => {
  router.push({
    name: 'SchemalessTableView', params: {
      uid: projectUID,
      viewUID: view.uid
    }
  })
}

const onEditView = (view: View) => {
  router.push({
    name: 'SchemalessViewSettings', params: {
      uid: route.params.uid,
      viewUID: view.uid
    }
  })
}

const onDeleteView = (view: View) => {
  const viewUID = view.uid
  deleteView(projectUID, viewUID).then(() => {
    views.value = views.value.filter(view => view.uid !== viewUID)
  }).then(() => {
    MessagePlugin.success('删除视图成功')
  }).finally(() => {
    getViews()
  })
}

onMounted(() => {
  getViews()
})
</script>

<style scoped>

</style>
