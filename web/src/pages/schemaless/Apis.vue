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

  <t-alert theme="info">
    API Endpoint：<code>{{ apiEndpointURL }}</code>
  </t-alert>

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
    <template #kind="{row}">
      <t-tag>
        {{ row.kind.toUpperCase() }}
      </t-tag>
    </template>
    <template #methods="{row}">
      <t-space size="5px">
        <t-tag v-for="method in row.methods" variant="light" :theme="{
          'GET': 'success',
          'POST': 'primary',
          'PUT': 'warning',
          'DELETE': 'danger',
        }[method as MethodType]">
          {{ method.toUpperCase() }}
        </t-tag>
      </t-space>
    </template>
    <template #path="{row}">
      <code> {{ row.path }} </code>
    </template>
    <template #params="{row}">
      <t-space>
        <t-tag v-for="param in row.queryParams" variant="light">
          GET: <code>{{ param.key }}</code>
        </t-tag>
        <t-tag v-for="param in row.bodyParams" variant="light">
          POST: <code>{{ param.key }}</code>
        </t-tag>
      </t-space>
    </template>
    <template #createdAt="{row}">
      {{ dayjs(row.createdAt).format('YYYY-MM-DD HH:mm:ss') }}
    </template>
    <template #ops="{row}">
      <t-space>
        <t-link theme="primary" @click="onViewApi(row)">编辑</t-link>
        <t-popconfirm theme="danger" content="你确定要删除该 API 吗？"
                      @confirm="onDeleteApi(row)">
          <t-link theme="danger" hover="color"> 删除</t-link>
        </t-popconfirm>
      </t-space>
    </template>
  </t-table>
</template>

<script setup lang="ts">
import {computed, onMounted, ref} from "vue";
import {useRoute, useRouter} from "vue-router";
import {listApis, type Api, MethodType, deleteApi} from "@/api/api";
import {
  MessagePlugin,
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
  {colKey: 'kind', title: '类型'},
  {colKey: 'methods', title: '请求方法'},
  {colKey: 'path', title: '路径'},
  {colKey: 'params', title: '请求参数'},
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

const apiEndpointURL = computed(() => {
  const baseURL = import.meta.env.VITE_API_BASE_URL.replace(/\/_$/, '')
  return `${baseURL}/api/${projectUID}`
})

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

const onDeleteApi = (api: Api) => {
  const apiUID = api.uid
  deleteApi(projectUID, apiUID).then(() => {
    apis.value = apis.value.filter(api => api.uid !== apiUID)
  }).then(() => {
    MessagePlugin.success('删除 API 成功')
  }).finally(() => {
    getApis()
  })
}

onMounted(() => {
  getApis()
})
</script>

<style scoped>

</style>
