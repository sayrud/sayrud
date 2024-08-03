<template>
  <t-row justify="space-between">
    <div class="operation-container">
      <t-button @click="createTableDialogVisible = true">
        <template #icon>
          <add-icon/>
        </template>
        新建数据表
      </t-button>
    </div>
  </t-row>

  <t-table
      :data="tables"
      :columns="COLUMNS"
      row-key="uid"
      vertical-align="top"
      :hover="true"
      :pagination="pagination"
      :loading="isLoading"
      @page-change="pagination = $event; getTables()"
  >
    <template #createdAt="{row}">
      {{ dayjs(row.createdAt).format('YYYY-MM-DD HH:mm:ss') }}
    </template>
    <template #ops="{row}">
      <t-space>
        <t-link theme="primary" @click="onViewTable(row)">编辑</t-link>
      </t-space>
    </template>
  </t-table>

  <t-dialog header="新建数据表" :close-btn="true" v-model:visible="createTableDialogVisible" cancel-btn="取消"
            @confirm="onConfirm">
    <template #body>
      <t-form
          ref="form"
          class="base-form"
          :data="formData"
          :rules="FORM_RULES"
          label-align="top"
          :label-width="100"
          @submit="onCreateTable"
      >
        <div class="form-basic-container">
          <div class="form-basic-item">
            <t-form-item label="数据表名" name="name">
              <t-input v-model="formData.name" placeholder="请输入数据表名"/>
            </t-form-item>
            <t-form-item label="数据表标签" name="label">
              <t-input v-model="formData.label" placeholder="请输入数据表标签"/>
            </t-form-item>
            <t-form-item label="数据表描述" name="desc">
              <t-textarea v-model="formData.desc" placeholder="请输入数据表描述"/>
            </t-form-item>
          </div>
        </div>
      </t-form>
    </template>
  </t-dialog>
  <AIFloat @refresh="getTables"/>
</template>

<script setup lang="ts">
import {onMounted, ref} from "vue";
import {useRoute, useRouter} from "vue-router";
import {listTables, createTable, type Table, CreateTableReq} from "@/api/schemalessTable.ts";
import {
  FormRule,
  MessagePlugin,
  type PaginationProps,
  PrimaryTableCol,
  SubmitContext,
  TableRowData
} from 'tdesign-vue-next';
import dayjs from 'dayjs'
import {AddIcon} from 'tdesign-icons-vue-next';
import AIFloat from "@/components/AIFloat.vue";

const route = useRoute()
const router = useRouter()

const projectUID = route.params.uid as string
const COLUMNS: PrimaryTableCol<TableRowData>[] = [
  {colKey: 'name', title: '表名'},
  {colKey: 'label', title: '标签'},
  {colKey: 'desc', title: '描述'},
  {colKey: 'count', title: '记录数'},
  {colKey: 'createdAt', title: '创建时间'},
  {colKey: 'ops', title: '操作'},
]
const isLoading = ref(false)
const pagination = ref<PaginationProps>({
  pageSize: 10,
  total: 0,
  current: 1,
})
const tables = ref<Table[]>([])
const getTables = () => {
  isLoading.value = true

  listTables(projectUID).then(res => {
    tables.value = res.tables
    pagination.value.total = res.total
  }).finally(() => {
    isLoading.value = false
  })
}

const createTableDialogVisible = ref<boolean>(false)
const form = ref()
const formData = ref<CreateTableReq>({
  name: '',
  label: '',
  desc: '',
})
const FORM_RULES: Record<string, FormRule[]> = {
  name: [{required: true, message: '请输入数据表名', type: 'error'}],
  label: [{required: true, message: '请输入数据表标签', type: 'error'}]
};
const onCreateTable = (ctx: SubmitContext) => {
  if (ctx.validateResult === true) {
    createTable(projectUID, formData.value).then(() => {
      createTableDialogVisible.value = false
      MessagePlugin.success('新建数据表成功')
      getTables()
    })
  }
}

const onConfirm = () => {
  form.value.submit()
}

const onViewTable = (table: Table) => {
  router.push({
    name: 'SchemalessTableSettings', params: {
      uid: route.params.uid,
      tableUID: table.uid
    }
  })
}
onMounted(() => {
  getTables()
})
</script>

<style scoped>

</style>
