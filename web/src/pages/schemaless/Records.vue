<template>
  <div class="table-tree-container">
    <div class="list-tree-wrapper">
      <div class="list-tree-operator">
        <t-list :split="true" :stripe="true">
          <t-list-item v-for="table in tables" v-bind:key="table.uid" @click="() => {
            currentTableUID = table.uid; getRecords()
          }" :class="['list-item', table.uid === currentTableUID ? 'active' : '']">
            <div style="display: flex; align-items: center; gap: 5px;">
              <Table1Icon/>
              {{ table.label }}
              <span style="color: var(--td-text-color-secondary)">{{ table.name }}</span>
            </div>
          </t-list-item>
        </t-list>
      </div>
      <div class="list-tree-content">
        <t-space direction="vertical">
          <t-button @click="onOpenDrawer">
            <template #icon>
              <add-icon/>
            </template>
            新建记录
          </t-button>
          <t-table
              :data="records"
              :columns="columns"
              row-key="uid"
              vertical-align="top"
              :hover="true"
              :pagination="pagination"
              :loading="isLoading"
              @page-change="pagination = $event; getRecords()"
          >
            <template #createdAt="{row}">
              {{ dayjs(row._created_at).format('YYYY-MM-DD HH:mm:ss') }}
            </template>
            <template #ops="{row}">
              <t-space>
                <t-link theme="primary" @click="onViewRecord(row._uid)">编辑</t-link>
              </t-space>
            </template>
          </t-table>
        </t-space>
      </div>
    </div>
  </div>

  <t-drawer v-model:visible="recordDrawerVisible" :header="formMode === 'create' ? '新建记录' : '修改记录'"
            size="medium" @confirm="onSubmit">
    <t-space direction="vertical" size="large" style="width: 100%" v-if="recordDrawerVisible">
      <t-space direction="vertical" :size="0" style="width: 100%" v-for="field in tableFields" v-bind:key="field.uid">
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
      </t-space>
    </t-space>
  </t-drawer>
</template>

<script setup lang="ts">
import {onMounted, ref} from "vue";
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
  updateRecord
} from "@/api/schemalessRecord";
import {AddIcon, Table1Icon} from 'tdesign-icons-vue-next';
import dayjs from "dayjs";
import {PaginationProps, PrimaryTableCol, TableRowData} from "tdesign-vue-next";

const route = useRoute()
const projectUID = route.params.uid as string

const tables = ref<Table[]>([])
const tableFields = ref<Field[]>([])
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
      }
    })

    BASE_COLUMNS.forEach(column => {
      columns.value.push({
        colKey: column.colKey,
        title: column.title as string,
      })
    })
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
  listRecord(projectUID, currentTableUID.value, {}).then(res => {
    records.value = res.records
    pagination.value.total = res.total
  }).finally(() => {
    isLoading.value = false
  })

  getTableFields()
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

const currentRecordUID = ref<string>('')
const onViewRecord = (uid: string) => {
  getRecord(projectUID, currentTableUID.value, uid).then(res => {
    formMode.value = 'update'
    currentRecordUID.value = uid
    recordFormData.value = res
    recordDrawerVisible.value = true
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

  .t-tree {
    margin-top: var(--td-comp-margin-xxl);
  }
}

.list-tree-wrapper {
  overflow-y: hidden;
}

.list-tree-operator {
  width: 280px;
  float: left;
  padding: 0 var(--td-comp-paddingTB-xxl) 0 var(--td-comp-paddingLR-xxl);
}

.list-tree-content {
  padding: 0 var(--td-comp-paddingTB-xxl) 0 var(--td-comp-paddingLR-xxl);
  border-left: 1px solid var(--td-border-level-1-color);
  overflow: auto;
}

.list-item {
  cursor: pointer;
  transition: background-color 0.2s;
  border-radius: var(--td-radius-medium);

  &:hover {
    background-color: var(--td-bg-color-component-hover);
  }

  &:active {
    background-color: var(--td-bg-color-component-active);
  }
}

.list-item.active {
  background-color: var(--td-bg-color-component-active);
}
</style>
