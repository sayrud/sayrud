<template>
  <t-form
      ref="mainForm"
      class="base-form"
      :data="formData"
      :rules="mode === 'create' ? CREATE_FORM_RULES : UPDATE_FORM_RULES"
      label-align="right"
      :label-width="70"
      @submit="onSubmit"
  >
    <div class="container">
      <div class="form-basic-item">
        <div class="form-basic-container-title">
          <div>
            {{ mode === 'create' ? '新建视图' : '编辑视图' }}
          </div>
        </div>
        <t-row class="row-gap" :gutter="[32, 24]">
          <t-col :span="4">
            <t-form-item label="名称" name="name">
              <t-input v-model="formData.name" placeholder="请输入视图名称"></t-input>
            </t-form-item>
          </t-col>
        </t-row>
        <t-row class="row-gap" :gutter="[32, 24]">
          <t-col :span="4">
            <t-form-item label="数据表" name="tableUID">
              <t-select v-model="formData.tableUID" placeholder="请选择数据表" @change="onSelectTable">
                <t-option v-for="table in tables" :key="table.uid" :value="table.uid"
                          :label="table.name">
                </t-option>
              </t-select>
            </t-form-item>
          </t-col>
          <t-col :span="4">
            <t-form-item label="字段" name="fieldUIDs">
              <t-select v-model="formData.fieldUIDs" placeholder="请选择字段" multiple>
                <t-option v-for="field in fields" :key="field.uid" :value="field.uid" :label="field.name">
                </t-option>
              </t-select>
            </t-form-item>
          </t-col>
        </t-row>
        <t-row class="row-gap" :gutter="[32, 24]">
          <t-col :span="4">
            <t-form-item label="筛选条件" name="filter">
              <t-textarea
                  v-model="filterData" placeholder="请输入筛选条件"
                  :autosize="{minRows: 6}">
              </t-textarea>
            </t-form-item>
          </t-col>
          <t-col :span="4">
            <t-form-item label="排序规则">
              <t-button variant="outline" @click="onAddOrderItem">
                <t-icon name="add"/>
              </t-button>
            </t-form-item>

            <t-form-item name="orderType" v-for="(_, index) in formData.order">
              <t-select v-model="formData.order[index].fieldUID">
                <t-option v-for="field in fields" :key="field.uid" :value="field.uid" :label="field.name">
                </t-option>
              </t-select>
              <t-select v-model="formData.order[index].orderType">
                <t-option value="asc" label="升序 A->Z"/>
                <t-option value="desc" label="降序 Z->A"/>
              </t-select>
              <template #statusIcon>
                <t-button variant="dashed" @click="onRemoveOrderItem(index)">
                  <t-icon name="remove"/>
                </t-button>
              </template>
            </t-form-item>
          </t-col>
        </t-row>
      </div>
    </div>
    <div class="form-submit-container">
      <div class="form-submit-sub">
        <t-space>
          <t-button theme="primary" class="form-submit-confirm" type="submit">确认提交</t-button>
        </t-space>
      </div>
    </div>
  </t-form>
</template>

<script setup lang="ts">
import {onMounted, ref} from "vue";
import {useRoute, useRouter} from "vue-router";
import {CreateViewReq, UpdateViewReq, createView, updateView, getView} from "@/api/schemalessView";
import {FormRule, MessagePlugin, type SubmitContext} from "tdesign-vue-next";
import {allTables, Table} from "@/api/schemalessTable.ts";
import {Field, listFields} from "@/api/schemalessField.ts";

const route = useRoute()
const router = useRouter()
const projectUID = route.params.uid as string
const viewUID = ref<string>(route.params.viewUID as string)
const CREATE_FORM_RULES: Record<string, FormRule[]> = {
  name: [{required: true, message: '请输入视图名称', type: 'error'}],
  tableUID: [{required: true, message: '请选择数据表', type: 'error'}],
  fieldUIDs: [{required: true, message: '请选择字段', type: 'error'}],
};
const UPDATE_FORM_RULES: Record<string, FormRule[]> = {
  name: [{required: true, message: '请输入视图名称', type: 'error'}],
  fieldUIDs: [{required: true, message: '请选择字段', type: 'error'}],
};

const tables = ref<Table[]>([])
const fields = ref<Field[]>([])

const filterData = ref<string>('')
const formData = ref<CreateViewReq | UpdateViewReq>({
  name: '',
  tableUID: '',
  fieldUIDs: [],
  filter: {},
  order: [],
})

const mode = ref<'create' | 'update'>('create')
if (route.name === 'SchemalessViewCreate') {
  mode.value = 'create'
} else if (route.name === 'SchemalessViewSettings') {
  mode.value = 'update'
}

const onSelectTable = (tableUID: string) => {
  listFields(projectUID, tableUID).then(res => {
    fields.value = res
  })
}

const onAddOrderItem = () => {
  formData.value.order.push({
    fieldUID: '',
    orderType: 'asc',
  })
}

const onRemoveOrderItem = (index: number) => {
  formData.value.order.splice(index, 1)
}

const onSubmit = (ctx: SubmitContext) => {
  if (ctx.validateResult === true) {
    try {
      if (filterData.value === '') {
        filterData.value = '{}'
      }
      formData.value.filter = JSON.parse(filterData.value)
    } catch (e) {
      MessagePlugin.error(`筛选条件解析失败 ${e}`)
      return
    }

    if (mode.value === 'create') {
      createView(projectUID, formData.value as CreateViewReq).then(res => {
        MessagePlugin.success('新建视图成功')
        router.push({name: 'SchemalessViewSettings', params: {uid: projectUID, viewUID: res.uid}})

        // Set the mode to update after creating the API.
        mode.value = 'update'
        viewUID.value = res.uid
      })
    } else if (mode.value === 'update') {
      updateView(projectUID, viewUID.value, formData.value as UpdateViewReq).then(() => {
        MessagePlugin.success('更新视图成功')
      })
    }
  }
}

onMounted(() => {
  allTables(projectUID).then(res => {
    tables.value = res
  })

  if (mode.value === 'update') {
    getView(projectUID, viewUID.value).then(res => {
      formData.value = {
        name: res.name,
        tableUID: res.table.uid,
        fieldUIDs: res.fieldUIDs,
        order: res.order,
        filter: res.filter,
      }

      onSelectTable(formData.value.tableUID)
      filterData.value = JSON.stringify(res.filter, null, 2)
    })
  }
})
</script>

<style scoped>
.container {
  display: flex;
  align-items: center;
  justify-content: left;
  background-color: var(--td-bg-color-container);
  border-radius: var(--td-radius-medium) var(--td-radius-medium) 0 0;
  padding: 0 0 80px 0;

  @media (max-width: @screen-sm-max) {
    padding: var(--td-comp-paddingTB-xl) var(--td-comp-paddingLR-xl) 80px var(--td-comp-paddingLR-xl);

    .form-basic-container-title {
      margin: 0 0 var(--td-comp-margin-xxxl) 0;
    }
  }

  .form-basic-item {
    width: 100%;

    .form-basic-container-title {
      font: var(--td-font-title-large);
      font-weight: 400;
      color: var(--td-text-color-primary);
      margin: var(--td-comp-margin-xxl) 0 var(--td-comp-margin-xl) 0;
      display: flex;
      justify-content: space-between;
    }
  }
}

.row-gap {
  margin-top: 20px;
}
</style>
