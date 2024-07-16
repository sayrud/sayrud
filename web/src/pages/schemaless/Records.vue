<template>
  <t-dialog v-model:visible="batchImportDialogVisible" header="批量添加" :confirm-btn="null" :cancel-btn="null">
    <t-form :label-width="0">
      <t-form-item>
        <t-textarea v-model="batchImportRaw" :autosize="{minRows: 6}"></t-textarea>
      </t-form-item>
      <t-form-item>
        <t-button @click="onBatchCreate">添加数据</t-button>
      </t-form-item>
    </t-form>
  </t-dialog>

  <div class="table-tree-container">
    <div class="list-tree-wrapper">
      <t-space direction="vertical" :size="0">
        <div class="list-header">
          <t-form-item>
            <t-select v-model="currentTableUID" @change="getRecords()">
              <t-option v-for="table in tables" v-bind:key="table.uid" :value="table.uid"
                        :label="table.name"></t-option>
            </t-select>
          </t-form-item>
          <t-space>
            <t-button theme="default" @click="batchImportDialogVisible = true">
              批量导入
            </t-button>
            <t-button @click="onOpenDrawer">
              <template #icon>
                <add-icon/>
              </template>
              新建记录
            </t-button>
          </t-space>
        </div>
        <t-table
            v-model:displayColumns="displayColumns"
            :column-controller="columnControllerConfig"
            :data="records"
            :columns="columns"
            row-key="uid"
            vertical-align="top"
            :hover="true"
            :pagination="pagination"
            :loading="isLoading"
            resizable
            @page-change="pagination = $event; getRecords()"
        >
          <template #createdAt="{row}">
            {{ dayjs(row._created_at).format('YYYY-MM-DD HH:mm:ss') }}
          </template>
          <template #ops="{row}">
            <t-space>
              <t-link theme="primary" @click="onViewRecord(row._uid)">编辑</t-link>
              <t-popconfirm theme="danger" content="你确定要删除该条记录吗？"
                            @confirm="onDeleteRecord(row._uid)">
                <t-link theme="danger" hover="color"> 删除</t-link>
              </t-popconfirm>
            </t-space>
          </template>
        </t-table>
      </t-space>
    </div>
  </div>

  <t-drawer v-model:visible="recordDrawerVisible" :header="formMode === 'create' ? '新建记录' : '修改记录'"
            size="medium" @confirm="onSubmit">
    <t-space direction="vertical" size="large" style="width: 100%" v-if="recordDrawerVisible">
      <t-space direction="vertical" :size="0" style="width: 100%"
               v-for="field in tableFields.filter(f => f.type !== FieldType.GENERATED)" v-bind:key="field.uid">
        <div style="font-weight: 600; margin-bottom: 5px;">{{ field.label }}</div>
        <t-input v-if="field.type === FieldType.TEXT" v-model="recordFormData.data[field.uid]"
                 :placeholder="`请输入${field.label}`"/>
        <t-input-number v-else-if="field.type === FieldType.INT || field.type === FieldType.FLOAT"
                        v-model="recordFormData.data[field.uid]"
                        :placeholder="`请输入${field.label}`"/>
        <t-switch v-else-if="field.type === FieldType.BOOL" v-model="recordFormData.data[field.uid]"/>
        <t-time-picker v-else-if="field.type === FieldType.DATE" v-model="recordFormData.data[field.uid]"
                       :placeholder="`请选择${field.label}`"></t-time-picker>
        <t-date-picker v-else-if="field.type === FieldType.TIMESTAMP"
                       v-model="recordFormData.data[field.uid]"></t-date-picker>
        <t-select v-else-if="field.type === FieldType.REFERENCE" v-model="recordFormData.data[field.uid]" filterable
                  :options="searchRecordOptions"
                  @search="(kw: string) => {queryRecords(field.options['reference_field_uid'] as string, kw)}"></t-select>
      </t-space>
    </t-space>
  </t-drawer>
</template>

<script setup lang="ts">
import {onMounted, ref, computed} from "vue";
import {useRoute} from "vue-router";
import {allTables, type Table} from "@/api/schemalessTable";
import {type Field, FieldType, listFields} from "@/api/schemalessField";
import {
  type CreateRecordReq,
  getRecord,
  listRecord,
  type Record,
  type UpdateRecordReq,
  createRecord,
  updateRecord,
  queryRecord,
  deleteRecord, batchCreateRecord, BatchCreateRecordReq
} from "@/api/schemalessRecord";
import {AddIcon} from 'tdesign-icons-vue-next';
import dayjs from "dayjs";
import {PaginationProps, PrimaryTableCol, TableRowData, MessagePlugin, type TableProps} from "tdesign-vue-next";

const route = useRoute()
const projectUID = route.params.uid as string

const tables = ref<Table[]>([])
const tableFields = ref<Field[]>([])
const displayColumns = ref<TableProps['displayColumns']>([]);
const columnControllerConfig = computed<TableProps['columnController']>(() => ({
  placement: 'top-right',
  dialogProps: {
    preventScrollThrough: true,
  },
}));
const currentTableUID = ref<string>('')
const BASE_COLUMNS: PrimaryTableCol<TableRowData>[] = [
  {colKey: 'createdAt', title: '创建时间'},
  {colKey: 'ops', title: '操作'},
]
const getTableFields = () => {
  listFields(projectUID, currentTableUID.value).then(res => {
    tableFields.value = res
    columns.value = res.map(field => {
      return {
        colKey: field.name,
        title: field.label,
        cell: (_: Function, {col, row}: { col: any, row: any }) => {
          if (field.type === FieldType.REFERENCE) {
            return JSON.parse(row[col.colKey])['v'] ? JSON.parse(row[col.colKey])['v'] : '-'
          } else {
            return row[col.colKey]
          }
        },
      }
    })

    columns.value.unshift({colKey: '_uid', title: 'UID'})

    BASE_COLUMNS.forEach(column => {
      columns.value.push({
        colKey: column.colKey,
        title: column.title as string,
      })
    })

    displayColumns.value = columns.value.map(item => item.colKey) as string[]
  })
}
const getTables = () => {
  allTables(projectUID).then(res => {
    tables.value = res

    if (currentTableUID.value === '' && res.length > 0) {
      currentTableUID.value = res[0].uid
      getRecords()
    }
  })
}

const isLoading = ref<boolean>(false)
const columns = ref<{ colKey?: string; title?: string }[]>([])
const records = ref<Record[]>([])
const pagination = ref<PaginationProps>({
  pageSize: 10,
  total: 0,
  current: 1,
})
const getRecords = () => {
  isLoading.value = true
  listRecord(projectUID, currentTableUID.value, {
    page: pagination.value.current,
    pageSize: pagination.value.pageSize,
  }).then(res => {
    records.value = res.records
    pagination.value.total = res.total
  }).finally(() => {
    isLoading.value = false
  })

  getTableFields()
}

const searchRecordOptions = ref<{ value: string; label: string }[]>([])
const queryRecords = (fieldUID: string, keyword: string) => {
  queryRecord(projectUID, currentTableUID.value, fieldUID, keyword).then(res => {
    searchRecordOptions.value = res.map(record => {
      return {value: record.uid, label: record.data[fieldUID]}
    })
  })
}

const formMode = ref<'create' | 'update'>('create')
const recordDrawerVisible = ref<boolean>(false)
const recordFormData = ref<CreateRecordReq | UpdateRecordReq>({} as CreateRecordReq)
const onOpenDrawer = () => {
  formMode.value = 'create'
  recordFormData.value = {data: {}}
  tableFields.value.forEach(field => {
    recordFormData.value.data[field.uid] = null
  })
  recordDrawerVisible.value = true
}

const batchImportDialogVisible = ref<boolean>(false)
const batchImportRaw = ref<string>('')
const onBatchCreate = () => {
  let data: { [key: string]: any }[] = []
  try {
    data = JSON.parse(batchImportRaw.value)
  } catch (e: Error) {
    MessagePlugin.error('数据格式错误 ' + e.toString())
    return
  }

  batchCreateRecord(projectUID, currentTableUID.value, {
    Data: data,
  }).then(() => {
    batchImportDialogVisible.value = false
  }).finally(() => {
    getRecords()
  })
}

const currentRecordUID = ref<string>('')
const onViewRecord = (uid: string) => {
  getRecord(projectUID, currentTableUID.value, uid).then(res => {
    formMode.value = 'update'
    currentRecordUID.value = uid
    recordFormData.value = res
    recordDrawerVisible.value = true
  })
}

const onDeleteRecord = (uid: string) => {
  deleteRecord(projectUID, currentTableUID.value, uid)
      .finally(() => {
        getRecords()
      })
}

const onSubmit = () => {
  if (formMode.value === 'create') {
    createRecord(projectUID, currentTableUID.value, recordFormData.value as CreateRecordReq).then(() => {
      recordDrawerVisible.value = false
    }).finally(() => {
      getRecords()
    })

  } else {
    updateRecord(projectUID, currentTableUID.value, currentRecordUID.value, recordFormData.value as UpdateRecordReq).then(() => {
      recordDrawerVisible.value = false
    }).finally(() => {
      getRecords()
    })
  }
}

onMounted(() => {
  getTables()
})
</script>

<style scoped>
.table-tree-container {
  background-color: var(--td-bg-color-container);
  border-radius: var(--td-radius-medium);
}

.list-tree-wrapper {
  overflow-y: hidden;

  .list-header {
    display: flex;
    justify-content: space-between;
  }
}
</style>
