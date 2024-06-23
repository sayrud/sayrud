<template>
  <t-row justify="space-between">
    <div class="operation-container">
      <t-button @click="onCreateApi">
        <template #icon>
          <add-icon/>
        </template>
        新建接口
      </t-button>
    </div>
  </t-row>

  <t-table
      :data="apis"
      :columns="COLUMNS"
      row-key="uid"
      vertical-align="top"
      :hover="true"
      :pagination="pagination"
      :loading="isLoading"
      @page-change="pagination = $event; getApis()"
  >
    <template #createdAt="{row}">
      {{ dayjs(row.createdAt).format('YYYY-MM-DD HH:mm:ss') }}
    </template>
    <template #ops="{row}">
      <t-space>
        <t-link theme="primary" @click="onViewApi(row)">编辑</t-link>
      </t-space>
    </template>
  </t-table>
</template>

<script setup lang="ts">
import {onMounted, ref} from "vue";
import {useRoute, useRouter} from "vue-router";
import {listApis, type Api} from "@/api/api";
import {
  type PaginationProps,
  PrimaryTableCol,
  TableRowData
} from 'tdesign-vue-next';
import dayjs from 'dayjs'
import {AddIcon} from 'tdesign-icons-vue-next';

const route = useRoute()
const router = useRouter()

const projectUID = route.params.uid as string
const COLUMNS: PrimaryTableCol<TableRowData>[] = [
  {colKey: 'createdAt', title: '创建时间'},
  {colKey: 'ops', title: '操作'},
]
const isLoading = ref(false)
const pagination = ref<PaginationProps>({
  pageSize: 10,
  total: 0,
  current: 1,
})
const apis = ref<Api[]>([])
const getApis = () => {
  isLoading.value = true

  listApis(projectUID, {
    page: pagination.value.current,
    pageSize: pagination.value.pageSize,
  }).then(res => {
    apis.value = res.apis
    pagination.value.total = res.total
  }).finally(() => {
    isLoading.value = false
  })
}

const onCreateApi = () => {
  router.push({name: 'SchemalessApiCreate', params: {uid: projectUID}})
}
const onViewApi = (api: Api) => {
  router.push({
    name: 'SchemalessApiSettings', params: {
      uid: route.params.uid,
      apiUID: api.uid
    }
  })
}
onMounted(() => {
  getApis()
})
</script>

<style scoped>

</style>
