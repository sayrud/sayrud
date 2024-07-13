<template>
  <div class="container">
    <div class="top">
      <div class="head">
        <div class="title">{{ table.label }}</div>
        <div class="subtitle">{{ table.name }}</div>
      </div>
      <div style="width: 80px; margin-right: 10px; color: var(--td-text-color-secondary)">
        {{ fields.length }} 个字段
      </div>
      <t-button @click="() => {formMode = 'create'; fieldsDialogVisible = true}">添加字段</t-button>
    </div>

    <t-form
        ref="form"
        class="form"
        :data="tableFormData"
        :rules="TABLE_FORM_RULES"
        label-align="top"
        :label-width="100"
        layout="inline"
        @submit="onUpdateTable"
    >
      <t-row class="row-gap" :gutter="[32, 24]">
        <t-col>
          <t-form-item label="数据表标签" name="label">
            <t-input v-model="tableFormData.label" placeholder="请输入数据表标签"/>
          </t-form-item>
        </t-col>
        <t-col>
          <t-form-item label="数据表描述" name="desc">
            <t-input v-model="tableFormData.desc" placeholder="请输入数据表描述"/>
          </t-form-item>
        </t-col>
        <t-col>
          <t-form-item label=" ">
            <t-button theme="primary" type="submit">保存</t-button>
          </t-form-item>
        </t-col>
      </t-row>
    </t-form>

    <t-table
        :data="fields"
        :columns="COLUMNS"
        row-key="uid"
        vertical-align="top"
        :hover="true"
        :loading="isLoading"
        dragSort='row-handler'
        @drag-sort="onFieldsDrag"
    >
      <template #drag>
        <div style="height: 100%; display: flex; align-items: center; justify-content: center;">
          <MoveIcon style="cursor: move"/>
        </div>
      </template>
      <template #no="{rowIndex}">
        {{ rowIndex + 1 }}
      </template>
      <template #type="{row}">
        {{ FieldTypeLabels[row.type] }}
      </template>
      <template #options="{row}">
        <t-space v-if="row.options">
          <t-tag v-for="k in Object.keys(row.options)">
            {{ k }} : {{
              k === 'reference_field_uid' ? fields.filter(f => f.uid === row.options[k])[0].name : row.options[k]
            }}
          </t-tag>
        </t-space>
      </template>
      <template #createdAt="{row}">
        {{ dayjs(row.createdAt).format('YYYY-MM-DD HH:mm:ss') }}
      </template>
      <template #ops="{row}">
        <t-space>
          <t-link theme="primary"
                  @click="() => {formMode = 'update'; fieldFormData = {fields: [JSON.parse(JSON.stringify(row))]}; fieldsDialogVisible = true}">
            编辑
          </t-link>
        </t-space>
      </template>
    </t-table>

    <t-dialog :header="formMode === 'create' ? '新建字段' : '编辑字段'" :close-btn="true"
              v-model:visible="fieldsDialogVisible" cancel-btn="取消"
              @confirm="onConfirm" width="750px">
      <template #body>
        <t-form
            ref="form"
            class="base-form"
            :data="fieldFormData"
            :rules="FIELD_FORM_RULES"
            :label-width="formMode === 'create' ? 0 : 120"
            @submit="onSubmitForm"
        >
          <div class="form-basic-item" v-if="formMode === 'create'">
            <t-form-item v-for="(_, index) in fieldFormData.fields" :key="index" label="">
              <t-space>
                <t-input v-model="fieldFormData.fields[index].name" placeholder="字段名"></t-input>
                <t-select v-model="fieldFormData.fields[index].type">
                  <t-option v-for="type in Object.values(FieldType)" :key="type" :value="type"
                            :label="FieldTypeLabels[type]"></t-option>
                </t-select>
                <t-input v-model="fieldFormData.fields[index].label" placeholder="标签"></t-input>
              </t-space>

              <template #statusIcon>
                <t-button v-if="index === 0" variant="dashed" @click="() => {fieldFormData.fields.push({} as Field)}">
                  <t-icon name="add"/>
                </t-button>
                <t-button variant="dashed" @click="fieldFormData.fields.splice(index, 1)">
                  <t-icon name="remove"/>
                </t-button>
              </template>
            </t-form-item>
          </div>

          <div class="form-basic-item" v-if="formMode === 'update'">
            <t-form-item label="字段名" name="name">
              <t-input v-model="fieldFormData.fields[0].name" placeholder="字段名"></t-input>
            </t-form-item>
            <t-form-item label="标签" name="label">
              <t-input v-model="fieldFormData.fields[0].label" placeholder="标签"></t-input>
            </t-form-item>
            <t-form-item v-for="(_, index) in fieldFormData.fields" :key="index" label="字段类型" name="type">
              <t-select v-model="fieldFormData.fields[0].type">
                <t-option v-for="type in Object.values(FieldType)" :key="type" :value="type"
                          :label="FieldTypeLabels[type]"></t-option>
              </t-select>
            </t-form-item>
            <t-form-item label="默认值" name="options.default"
                         v-if="![FieldType.REFERENCE, FieldType.GENERATED].includes(fieldFormData.fields[0].type)"
                         tips="支持表达式">
              <t-input v-model="fieldFormData.fields[0].options['default']"></t-input>
            </t-form-item>
            <!-- REFERENCE -->
            <t-form-item v-if="fieldFormData.fields[0].type === FieldType.REFERENCE" label="引用列"
                         name="options.reference">
              <t-select v-model="fieldFormData.fields[0].options['reference_field_uid']">
                <t-option v-for="field in fields.filter(f => f.uid !== fieldFormData.fields[0].uid)" :key="field.uid"
                          :value="field.uid" :label="field.label">
                </t-option>
              </t-select>
            </t-form-item>
            <t-form-item v-if="fieldFormData.fields[0].type === FieldType.REFERENCE" label="约束"
                         name="options.constraint">
              <t-input v-model="fieldFormData.fields[0].options['constraint']"/>
            </t-form-item>
            <!-- GENERATED -->
            <t-form-item v-if="fieldFormData.fields[0].type === FieldType.GENERATED" label="表达式"
                         name="options.expression" tips="PostgreSQL 语法">
              <t-input v-model="fieldFormData.fields[0].options['expression']"></t-input>
            </t-form-item>
          </div>

        </t-form>
      </template>
    </t-dialog>

  </div>
</template>

<script setup lang="ts">
import {onMounted, ref} from 'vue'
import {useRoute} from "vue-router";
import {getTable, Table, updateTable, UpdateTableReq} from "@/api/schemalessTable.ts";
import NProgress from "nprogress";
import {FormRule, MessagePlugin, PrimaryTableCol, SubmitContext, TableRowData} from "tdesign-vue-next";
import dayjs from "dayjs";
import {
  type CreateFieldReq,
  type Field,
  FieldType,
  listFields,
  type UpdateFieldReq,
  createFields,
  updateFields, FieldTypeLabels
} from "@/api/schemalessField.ts";
import {MoveIcon} from 'tdesign-icons-vue-next';

const route = useRoute()
const projectUID = route.params.uid as string
const tableUID = route.params.tableUID as string

const TABLE_FORM_RULES = {
  label: [{required: true, message: '请输入数据表标签', type: 'error'}],
}
const table = ref<Table>({} as Table)
const tableFormData = ref<UpdateTableReq>({} as UpdateTableReq)

const onUpdateTable = () => {
  updateTable(projectUID, tableUID, tableFormData.value).then(() => {
    MessagePlugin.success('保存成功')
  }).finally(() => {
    refreshTable()
  })
}

const refreshTable = () => {
  NProgress.start()
  getTable(projectUID, tableUID).then(res => {
    table.value = res
    tableFormData.value = JSON.parse(JSON.stringify(res))
  }).finally(() => {
    NProgress.done()
  })
}

const isLoading = ref<boolean>(false)
const fields = ref<Field[]>([])
const COLUMNS: PrimaryTableCol<TableRowData>[] = [
  {colKey: 'drag', width: 60},
  {colKey: 'no', title: '序号', width: 80},
  {colKey: 'name', title: '字段名'},
  {colKey: 'label', title: '标签'},
  {colKey: 'type', title: '类型', width: 100},
  {colKey: 'options', title: '选项'},
  {colKey: 'ops', title: '操作', width: 200},
]
const refreshFields = () => {
  isLoading.value = true
  listFields(projectUID, tableUID).then(res => {
    fields.value = res
  }).finally(() => {
    isLoading.value = false
  })
}

const onFieldsDrag = (params: any) => {
  const current = params.current
  const target = params.target

  const currentPositionIndex = current.position
  const targetPositionIndex = target.position
  current.position = targetPositionIndex
  target.position = currentPositionIndex

  updateFields(projectUID, tableUID, {
    fields: [current, target]
  }).finally(() => {
    refreshFields()
  })
}

const form = ref()
const formMode = ref<'create' | 'update'>('create')
const fieldsDialogVisible = ref<boolean>(false)
const fieldFormData = ref<CreateFieldReq | UpdateFieldReq>({
  fields: [
    {name: '', label: '', type: FieldType.TEXT, options: {}}
  ]
})
const FIELD_FORM_RULES: Record<string, FormRule[]> = {
  fields: [{required: true, message: '请填写字段', type: 'error'}],
};

const onConfirm = () => {
  form.value.submit()
}

const onSubmitForm = (ctx: SubmitContext) => {
  if (ctx.validateResult === true) {
    if (formMode.value === 'create') {
      createFields(projectUID, tableUID, fieldFormData.value).then(() => {
        fieldsDialogVisible.value = false
        MessagePlugin.success('新建字段成功')
        fieldFormData.value.fields = [{name: '', label: '', type: FieldType.TEXT, options: {}}]
        refreshFields()
      })
    } else {
      updateFields(projectUID, tableUID, fieldFormData.value as UpdateFieldReq).then(() => {
        fieldsDialogVisible.value = false
        MessagePlugin.success('编辑字段成功')
        fieldFormData.value.fields = [{name: '', label: '', type: FieldType.TEXT, options: {}}]
        refreshFields()
      })
    }
  }
}

onMounted(() => {
  refreshTable()
  refreshFields()
})

</script>

<style lang="less" scoped>
.container {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  background-color: var(--td-bg-color-container);
  border-radius: var(--td-radius-medium) var(--td-radius-medium) 0 0;
  padding: 0 var(--td-comp-paddingLR-xxl) 80px var(--td-comp-paddingLR-xxl);
  gap: 15px;

  .top {
    width: 100%;
    display: flex;
    flex-direction: row;
    align-items: center;
    justify-content: space-between;
  }

  .head {
    width: 100%;
    display: flex;
    flex-direction: row;
    align-items: center;
    gap: 20px;
  }

  .form {
    width: 100%;
  }
}

.title {
  font: var(--td-font-title-large);
  font-weight: 400;
  color: var(--td-text-color-primary);
}

.subtitle {
  font: var(--td-font-title-large);
  color: var(--td-text-color-secondary);
}
</style>
