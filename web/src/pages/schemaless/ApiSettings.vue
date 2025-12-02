<template>
  <t-row :gutter="16">
    <t-col :span="9">
      <t-form
          ref="mainForm"
          class="base-form"
          :data="formData"
          :rules="FORM_RULES"
          label-align="right"
          :label-width="70"
      >
        <div class="container">
          <div class="form-basic-item">
            <div class="form-basic-container-title">
              <div>
                {{ mode === 'create' ? '新建接口' : '编辑接口' }}
                <t-loading v-if="isSaving" text="保存中..." size="small"/>
              </div>
              <t-button theme="primary" @click="onSubmit">保存</t-button>
            </div>
            <t-row class="row-gap" :gutter="[32, 24]">
              <t-col :span="4">
                <t-form-item label="接口类型" name="kind">
                  <t-select v-model="formData.kind" placeholder="请选择接口类型">
                    <t-option v-for="kind in ['list','view','create','update', 'delete']" :key="kind" :value="kind"
                              :label="kind.toUpperCase()">
                    </t-option>
                  </t-select>
                </t-form-item>
              </t-col>
              <t-col :span="4">
                <t-form-item label="请求方法" name="methods">
                  <t-select v-model="formData.methods" placeholder="请选择请求方法" multiple>
                    <t-option v-for="method in ['GET','POST','PUT','DELETE']" :key="method" :value="method">
                      {{ method }}
                    </t-option>
                  </t-select>
                </t-form-item>
              </t-col>
              <t-col :span="4">
                <t-form-item label="路径" name="path">
                  <t-input v-model="formData.path" :style="{ width: '322px' }" placeholder="请输入请求路径"/>
                </t-form-item>
              </t-col>
            </t-row>
            <t-row class="row-gap" :gutter="[32, 24]">
              <t-col :span="12">
                <t-form-item label="请求参数">
                  <t-space direction="vertical" style="width: 100%">
                    <div style="width: 16px; display: flex; justify-content: center; margin-left: 8px;">
                      <t-button theme="default" variant="outline" @click="onRequestParamsAddRow">
                        <t-icon name="add"/>
                      </t-button>
                    </div>
                    <t-table
                        ref="paramsTableRef"
                        row-key="no"
                        :columns="paramsColumns"
                        :data="requestParams"
                        :editable-row-keys="requestParamsEditableRowKeys"
                        table-layout="auto"
                        size="small"
                        bordered
                        lazy-load
                        @row-edit="onParamsRowEdit"
                        @row-validate="onParamsRowValidate"
                        @validate="onParamsValidate"
                    >
                      <template #no="{rowIndex}">
                        {{ rowIndex + 1 }}
                      </template>
                      <template #empty>
                        无请求参数
                      </template>
                      <template #kind="{row}">
                        <t-tag>
                          {{ ParamKindLabels[row.kind as ParamKind] }}
                        </t-tag>
                      </template>
                      <template #type="{row}">
                        {{ ParamTypeLabels[row.type as ParamType] }}
                      </template>
                      <template #required="{row}">
                        {{ row.required ? '是' : '否' }}
                      </template>
                      <template #customValidators="{row}">
                        <t-link theme="primary" @click="onEditValidators(row)">
                          {{ row.customValidators && row.customValidators.length ? `已配置 ${row.customValidators.length} 条` : '配置' }}
                        </t-link>
                      </template>
                      <template #ops="{row}">
                        <t-space>
                          <t-link v-if="!requestParamsEditableRowKeys.includes(row.no)" theme="primary" hover="color"
                                  @click="onRequestParamsEditRow(row.no)">编辑
                          </t-link>
                          <t-link v-if="requestParamsEditableRowKeys.includes(row.no)" theme="primary" hover="color"
                                  @click="onRequestParamsSaveRow(row.no)">保存
                          </t-link>
                          <t-link v-if="requestParamsEditableRowKeys.includes(row.no)" theme="primary" hover="color"
                                  @click="onRequestParamsCancelRow(row.no)">取消
                          </t-link>
                          <t-popconfirm theme="danger" content="你确定要删除该参数吗？"
                                        @confirm="onRequestParamsDeleteRow(row.no)">
                            <t-link theme="danger" hover="color"> 删除</t-link>
                          </t-popconfirm>
                        </t-space>
                      </template>
                    </t-table>
                  </t-space>
                </t-form-item>
              </t-col>
              <t-col :span="12">
                <t-row :gutter="[32, 24]">
                  <t-col :span="6">
                    <t-form-item label="数据表"
                                 tips="Query参数：$request.query /  Body参数：$request.body">
                      <t-space direction="vertical" style="width: 100%">
                        <t-select v-model="datasets[0].tableUID" placeholder="请选择数据表" @change="onSelectDataset">
                          <t-option v-for="table in tables" :key="table.uid" :value="table.uid"
                                    :label="table.name"></t-option>
                        </t-select>
                        <t-textarea
                            v-if="formData.kind !== 'create'"
                            v-model="filterData" placeholder="请输入筛选条件"
                            :autosize="{minRows: 6}"></t-textarea>
                      </t-space>
                    </t-form-item>

                    <t-form-item label="排序字段" v-if="formData.kind === 'list' && datasets[0]">
                      <t-button variant="outline" @click="onAddOrderItem">
                        <t-icon name="add"/>
                      </t-button>
                    </t-form-item>
                    <t-form-item name="orderType" v-for="(_, index) in datasets[0].order"
                                 v-if="formData.kind === 'list' && datasets[0]">
                      <t-select v-model="datasets[0]['order'][index].fieldUID">
                        <t-option v-for="field in tableFieldsMap[datasets[0].tableUID]" :key="field.value"
                                  :value="field.value" :label="field.label">
                        </t-option>
                      </t-select>
                      <t-select v-model="datasets[0]['order'][index].orderType">
                        <t-option value="asc" label="升序 A->Z"/>
                        <t-option value="desc" label="降序 Z->A"/>
                      </t-select>
                      <template #statusIcon>
                        <t-button variant="dashed" @click="onRemoveOrderItem(index)">
                          <t-icon name="remove"/>
                        </t-button>
                      </template>
                    </t-form-item>

                    <t-form-item label="LIMIT" v-if="formData.kind === 'list'">
                      <t-input v-model="datasets[0].limitExp" placeholder="请输入LIMIT"/>
                    </t-form-item>
                    <t-form-item label="OFFSET" v-if="formData.kind === 'list'">
                      <t-input v-model="datasets[0].offsetExp" placeholder="请输入OFFSET"/>
                    </t-form-item>
                  </t-col>
                  <t-col :span="6">
                    <t-form-item label="字段映射" v-if="formData.kind === 'create' || formData.kind === 'update'">
                      <t-space direction="vertical">
                        <t-row v-for="param in requestParams" v-bind:key="`${param.kind}:${param.key}`">
                          <t-tag>{{ param.kind }} : {{ param.key }}</t-tag>
                          <t-select
                              v-if="datasets && datasets[0]"
                              v-model="(formData.fieldMapping as Record<string, string>)[`${param.kind}:${param.key}`]"
                              placeholder="请选择字段">
                            <t-option v-for="field in tableFieldsMap[datasets[0].tableUID]" :key="field.value"
                                      :value="field.value"
                                      :label="field.label"></t-option>
                          </t-select>
                        </t-row>
                      </t-space>
                    </t-form-item>

                    <t-form-item label="字段" v-if="formData.kind === 'list' || formData.kind === 'view'">
                      <t-select
                          v-if="datasets && datasets[0]"
                          v-model="datasets[0].fields"
                          placeholder="请选择字段" multiple>
                        <t-option v-for="field in tableFieldsMap[datasets[0].tableUID]" v-bind:key="field.value"
                                  :value="field.value"
                                  :label="field.label"></t-option>
                      </t-select>
                    </t-form-item>

                    <t-form-item label="字段别名" v-if="formData.kind === 'list' || formData.kind === 'view'">
                      <t-space direction="vertical">
                        <t-row v-for="field in datasets[0].fields" v-bind:key="field">
                          <t-tag>
                            {{ tableFieldsMap[datasets[0].tableUID].filter(item => item.value === field)[0].label }}
                          </t-tag>
                          <t-input v-model="(formData.fieldMapping as Record<string, string>)[field]"
                                   placeholder="请输入字段别名"/>
                        </t-row>
                      </t-space>
                    </t-form-item>
                  </t-col>
                </t-row>
              </t-col>
            </t-row>
            <t-row class="row-gap" :gutter="[32, 24]">
              <t-col :span="12">
                <t-form-item label="响应格式">
                  <t-textarea v-model="formData.response" placeholder="请输入响应格式"/>
                </t-form-item>
              </t-col>
            </t-row>
          </div>
        </div>
      </t-form>
    </t-col>
    <t-col :span="3">
      <div class="side">
        <Container orientation="vertical"
                   class="container"
                   :drop-placeholder="{
                      className: 'drop-placeholder',
                      animationDuration: '200',
                      showOnTop: true
                   }"
                   drag-class="drag"
                   drop-class="drop"
                   @drop="onDropMiddlewares">
          <Draggable v-for="(item, index) in formData.middlewares" :key="index" style="z-index: 2">
            <t-popup class="placement align" placement="left" show-arrow destroy-on-close>
              <template #content>
                <t-card :bordered="false">
                  <t-form
                      v-if="item.type !== 'main'"
                      class="base-form"
                      label-align="right"
                      :label-width="120"
                  >
                    <!-- rate limit -->
                    <div v-if="item.type === 'rate_limit'">
                      <t-form-item label="限流策略">
                        <t-select size="small" v-model="formData.middlewares[index].params['policy']"
                                  placeholder="请选择限流策略">
                          <t-option value="period" label="周期请求次数"></t-option>
                        </t-select>
                      </t-form-item>
                      <t-form-item label="周期（秒）">
                        <t-input-number size="small" v-model="formData.middlewares[index].params['period']"/>
                      </t-form-item>
                      <t-form-item label="最大请求次数">
                        <t-input-number size="small" v-model="formData.middlewares[index].params['value']"/>
                      </t-form-item>
                    </div>

                    <!-- turnstile_captcha -->
                    <div v-if="item.type === 'turnstile_captcha'">
                      <t-form-item label="Secret Key">
                        <t-input size="small" v-model="formData.middlewares[index].params['secretKey']"></t-input>
                      </t-form-item>
                    </div>
                  </t-form>
                  <span v-else>无操作选项</span>
                </t-card>
              </template>

              <div :class="['node',item.type === 'main'? 'main': '']">
                <div class="delete-icon" @click="onDeleteMiddleware(index)" v-if="item.type !== 'main'">
                  <t-icon :size="18" name="close-circle-filled"/>
                </div>
                <div class="inner">
                  <t-icon :name="MiddlewareIcons[item.type]"/>
                  {{ MiddlewareNames[item.type] }}
                </div>
              </div>
            </t-popup>
          </Draggable>
        </Container>
        <div class="line" :style="{height: `${formData.middlewares.length*80}px`}"></div>
      </div>

      <div class="tool">
        <t-popup placement="bottom" show-arrow destroy-on-close>
          <template #content>
            <t-list :split="true" size="small">
              <t-list-item v-for="(name, type) in MiddlewareSelectList">
                <div style="display: flex; gap: 5px; align-items: center; cursor: pointer"
                     @click="onSelectMiddleware(type)">
                  <t-icon :name="MiddlewareIcons[type]"/>
                  {{ name }}
                </div>
              </t-list-item>
            </t-list>
          </template>

          <t-button shape="circle" theme="primary">
            <template #icon>
              <add-icon/>
            </template>
          </t-button>
        </t-popup>
      </div>
    </t-col>
  </t-row>

  <t-dialog
      v-model:visible="validatorDialogVisible"
      header="自定义校验规则配置"
      width="800px"
      @confirm="onValidatorDialogConfirm"
  >
    <t-button theme="primary" style="margin-bottom: 16px" @click="onAddValidator">
      <template #icon>
        <t-icon name="add"/>
      </template>
      添加规则
    </t-button>

    <t-table
        :data="currentValidators"
        :columns="validatorColumns"
        row-key="_id"
        size="small"
        bordered
    >
      <template #type="{ row }">
        <t-input v-model="row.type" placeholder="请输入类型"/>
      </template>
      <template #expression="{ row }">
        <t-input v-model="row.expression" placeholder="请输入表达式"/>
      </template>
      <template #message="{ row }">
        <t-input v-model="row.message" placeholder="请输入错误提示"/>
      </template>
      <template #ops="{ rowIndex }">
        <t-link theme="danger" @click="onRemoveValidator(rowIndex)">删除</t-link>
      </template>
    </t-table>
  </t-dialog>
</template>

<script setup lang="ts">
import {computed, onMounted, onUnmounted, ref, watch} from 'vue'
import {useRoute, useRouter} from "vue-router";
import {
  FormRule,
  Input,
  MessagePlugin,
  Select,
  Switch,
  type TableInstanceFunctions,
  type TableProps,
  TableRowData,
} from "tdesign-vue-next";
import NProgress from "nprogress";
import {
  createApi,
  CreateApiReq,
  Dataset,
  getApi,
  ParamKind,
  ParamKindLabels,
  ParamKindOptions,
  ParamMixin,
  ParamType,
  ParamTypeLabels,
  ParamTypeOptions,
  updateApi,
  UpdateApiReq,
  Validator,
} from "@/api/api.ts";
import {Container, Draggable} from "vue3-smooth-dnd";
import {AddIcon} from 'tdesign-icons-vue-next';
import {allTables, type Table} from '@/api/schemalessTable'
import {listFields} from "@/api/schemalessField";
import {MiddlewareIcons, MiddlewareNames, MiddlewareSelectList} from "@/const/middlewares.ts";
import {nanoid} from 'nanoid'

const route = useRoute()
const router = useRouter()
const mainForm = ref()
const mode = ref<'create' | 'update'>('create')
const projectUID = route.params.uid as string
const apiUID = ref<string>(route.params.apiUID as string)
if (route.name === 'SchemalessApiCreate') {
  mode.value = 'create'
} else if (route.name === 'SchemalessApiSettings') {
  mode.value = 'update'
}
const tables = ref<Table[]>([])

const FORM_RULES: Record<string, FormRule[]> = {
  name: [{required: true, message: '请输入项目名', type: 'error'}],
  schemaName: [{required: true, message: '请输入项目ID', type: 'error'}],
  response: [{required: true, message: '请输入响应格式', type: 'error'}],
};

// Request params table
const paramsTableRef = ref<TableInstanceFunctions>()
const paramsColumns = computed<TableProps['columns']>(() => [
  {colKey: 'no', title: '#', width: 50, align: 'center'},
  {
    colKey: 'kind', title: '参数类型', width: 100, align: 'center', edit: {
      component: Select,
      props: {clearable: true, autoWidth: true, size: 'small', options: ParamKindOptions},
      rules: [{required: true, message: '不能为空'}],
      showEditIcon: false
    }
  },
  {
    colKey: 'key', title: '参数名', width: 150, align: 'center', edit: {
      component: Input,
      props: {clearable: true, autofocus: true, autoWidth: true, size: 'small'},
      rules: [{required: true, message: '不能为空'}],
      showEditIcon: false
    }
  },
  {
    colKey: 'label', title: '参数标签', width: 150, align: 'center', edit: {
      component: Input,
      props: {clearable: true, autoWidth: true, size: 'small'},
      showEditIcon: false
    }
  },
  {
    colKey: 'type', title: '类型', width: 120, align: 'center', edit: {
      component: Select,
      props: {clearable: true, autoWidth: true, size: 'small', options: ParamTypeOptions},
      rules: [{required: true, message: '不能为空'}],
      showEditIcon: false
    }
  },
  {
    colKey: 'required', title: '是否必填', width: 100, align: 'center', edit: {
      component: Switch,
      showEditIcon: false
    }
  },
  {
    colKey: 'customValidators', title: '自定义校验规则', align: 'center', width: 150
  },
  {colKey: 'ops', title: '操作', width: 150, align: 'center',},
])
const requestParamsEditMap: Record<string, TableRowData> = {}
const requestParams = ref<ParamMixin[]>([{
  no: nanoid(),
  kind: 'query',
  key: 'name',
  label: '名称',
  type: 'string',
  required: false,
  customValidators: [],
}])
const requestParamsEditableRowKeys = ref<string[]>([])

const onRequestParamsAddRow = () => {
  requestParams.value.push({
    no: nanoid(),
    kind: 'query',
    key: '',
    label: '',
    type: 'string',
    required: false,
    customValidators: [],
  })
}
const onRequestParamsEditRow = (key: string) => {
  if (!requestParamsEditableRowKeys.value.includes(key)) {
    requestParamsEditableRowKeys.value.push(key)
    paramsTableRef.value?.clearValidateData()
  }
}

const onRequestParamsSaveRow = (key: string) => {
  paramsTableRef.value?.validateRowData(key).then((params) => {
    if (params.result.length) {
      const r = params.result[0]
      MessagePlugin.error(`${r.col.title} ${r.errorList[0].message}`)
      return
    }

    // Invoked by the table component.
    if (params.trigger === 'parent' && !params.result.length) {
      const current = requestParamsEditMap[key]
      if (current) {
        console.log(current)
        const rowIndex = requestParams.value.findIndex(param => param.no === key)
        requestParams.value.splice(rowIndex, 1, current.editedRow)
      }
      const rowIndex = requestParamsEditableRowKeys.value.findIndex(k => k === key)
      requestParamsEditableRowKeys.value.splice(rowIndex, 1)
    }
  })
}

const onRequestParamsCancelRow = (key: string) => {
  const rowIndex = requestParamsEditableRowKeys.value.findIndex(k => k === key)
  requestParamsEditableRowKeys.value.splice(rowIndex, 1)
}

const onRequestParamsDeleteRow = (key: string) => {
  const rowIndex = requestParams.value.findIndex(param => param.no === key)
  requestParams.value.splice(rowIndex, 1)
}
const onParamsRowEdit: TableProps['onRowEdit'] = (params) => {
  const {row, col, value} = params;
  const oldRowData = requestParamsEditMap[row.no]?.editedRow || row;
  const editedRow = {
    ...oldRowData,
    [col.colKey as string]: value,
  }
  requestParamsEditMap[row.no] = {
    ...params,
    editedRow,
  }
}

const onParamsRowValidate = () => {

}

const onParamsValidate = () => {

}

// Custom Validators Dialog
const validatorDialogVisible = ref(false)
const currentValidators = ref<(Validator & { _id: string })[]>([])
const currentEditingParamNo = ref('')
const validatorColumns = [
  { colKey: 'type', title: '类型', width: 150 },
  { colKey: 'expression', title: '表达式' },
  { colKey: 'message', title: '错误提示' },
  { colKey: 'ops', title: '操作', width: 100, align: 'center' },
]

const onEditValidators = (row: ParamMixin) => {
  currentEditingParamNo.value = row.no!
  currentValidators.value = (row.customValidators || []).map(v => ({
    ...v,
    _id: nanoid()
  }))
  validatorDialogVisible.value = true
}

const onValidatorDialogConfirm = () => {
  const index = requestParams.value.findIndex(p => p.no === currentEditingParamNo.value)
  if (index !== -1) {
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    requestParams.value[index].customValidators = currentValidators.value.map(({_id, ...v}) => v)
  }
  validatorDialogVisible.value = false
}

const onAddValidator = () => {
  currentValidators.value.push({
    type: '',
    expression: '',
    message: '',
    _id: nanoid()
  })
}

const onRemoveValidator = (index: number) => {
  currentValidators.value.splice(index, 1)
}

const datasets = ref<Dataset[]>([{} as Dataset])
const datasetsNames = ref<Record<string, string>>({})
const tableFieldsMap = ref<Record<string, { value: string; label: string }[]>>({})
const onSelectDataset = (tableUID: string) => {
  if (!tableUID) {
    return
  }

  // Clean the previous field mapping.
  formData.value.fieldMapping = {}

  listFields(projectUID, tableUID).then(res => {
    tableFieldsMap.value[tableUID] = res.map(field => ({
      value: field.uid,
      label: field.name,
    }))
  })
  tableFieldsMap.value[tableUID].push({
    value: '_uid',
    label: '_uid'
  })
}

const filterData = ref<string>('')
const formData = ref<CreateApiReq | UpdateApiReq>({
  kind: 'list',
  methods: [],
  path: '',
  queryParams: [],
  bodyParams: [],
  fieldMapping: {},
  datasets: [],
  middlewares: [{
    type: 'main', params: {}
  }],
  response: '',
})

const onAddOrderItem = () => {
  datasets.value[0].order.push({
    fieldUID: '',
    orderType: 'asc',
  })
}

const onRemoveOrderItem = (index: number) => {
  datasets.value[0].order.splice(index, 1)
}

const onSelectMiddleware = (type: string) => {
  formData.value.middlewares.push({
    type: type,
    params: {}
  })
}

const onDeleteMiddleware = (index: number) => {
  formData.value.middlewares.splice(index, 1)
}

const onDropMiddlewares = (dropResult: any) => {
  const {removedIndex, addedIndex, payload} = dropResult;

  if (removedIndex === null && addedIndex === null) {
    return
  }
  const result = [...formData.value.middlewares];
  let itemToAdd = payload;

  if (removedIndex !== null) {
    itemToAdd = result.splice(removedIndex, 1)[0];
  }
  if (addedIndex !== null) {
    result.splice(addedIndex, 0, itemToAdd);
  }
  formData.value.middlewares = result
}

const isSaving = ref<boolean>(false)
const onSubmit = async () => {
  const validateResult = await mainForm.value.validate()
  if (validateResult === true) {
    isSaving.value = true
    NProgress.start()

    // Convert the request params to the correct format.
    formData.value.queryParams = requestParams.value.filter(param => param.kind === 'query')
    formData.value.bodyParams = requestParams.value.filter(param => param.kind === 'body')

    // Convert the datasets to the correct format.
    formData.value.datasets = datasets.value

    let filterObject = {} as any
    if (filterData.value) {
      try {
        filterObject = JSON.parse(filterData.value)
      } catch (e) {
        MessagePlugin.error(`筛选条件解析失败 ${e}`)
        return
      }

      if (formData.value.kind === 'list' || formData.value.kind === 'view') {
        formData.value.datasets[0].filter = filterObject
      } else if (formData.value.kind === 'update' || formData.value.kind === 'delete') {
        formData.value.filter = filterObject
      }
    }

    if (mode.value === 'create') {
      createApi(projectUID, {
        ...formData.value,
      } as CreateApiReq).then(res => {
        MessagePlugin.success('新建接口成功')
        router.push({name: 'SchemalessApiSettings', params: {uid: projectUID, apiUID: res.uid}})

        // Set the mode to update after creating the API.
        mode.value = 'update'
        apiUID.value = res.uid
      }).finally(() => {
        NProgress.done()
        isSaving.value = false
      })

    } else {
      updateApi(projectUID, apiUID.value, {
        ...formData.value,
      } as UpdateApiReq).then(() => {
      }).finally(() => {
        NProgress.done()
        isFormChanged.value = false
        isSaving.value = false
      })
    }
  }
};

// Autosave
const autoSaveTimer = ref<number | null>(null)
const isFormChanged = ref<boolean>(false)
watch(formData, () => {
  isFormChanged.value = true
}, {deep: true})
watch(requestParams, () => {
  isFormChanged.value = true
}, {deep: true})
watch(datasets, () => {
  isFormChanged.value = true
}, {deep: true})


onMounted(() => {
  allTables(projectUID).then(res => {
    tables.value = res
    datasetsNames.value = res.reduce((acc, table) => {
      acc[table.uid] = table.name
      return acc
    }, {} as Record<string, string>)
  })

  if (mode.value === 'update') {
    getApi(projectUID, apiUID.value).then(async res => {
      const params = []
      if (res.queryParams) {
        for (const key in res.queryParams) {
          params.push({
            kind: 'query',
            ...res.queryParams[key]
          })
        }
      }
      if (res.bodyParams) {
        for (const key in res.bodyParams) {
          params.push({
            kind: 'body',
            ...res.bodyParams[key]
          })
        }
      }
      requestParams.value = params.map(param => ({
        no: nanoid(),
        ...param
      }))

      // Load the datasets' fields.
      if ((res.kind === 'list' || res.kind === 'view') && res.options.datasets) {
        for (const dataset of res.options.datasets) {
          const tableUID = dataset.tableUID
          const fields = await listFields(projectUID, tableUID)
          tableFieldsMap.value[tableUID] = fields.map(field => ({
            value: field.uid,
            label: field.name,
          }))
          tableFieldsMap.value[tableUID].push({value: '_uid', label: '_uid'})
        }
        datasets.value = res.options.datasets

      } else if (res.kind === 'create' || res.kind === 'update' || res.kind === 'delete') {
        const tableUID = res.options.tableUID
        const fields = await listFields(projectUID, tableUID)
        tableFieldsMap.value[tableUID] = fields.map(field => ({
          value: field.uid,
          label: field.name,
        }))
        tableFieldsMap.value[tableUID].push({value: '_uid', label: '_uid'})
        datasets.value = [{tableUID: tableUID} as Dataset]
      }

      const fieldMapping = res.options?.fieldMapping


      if (res.kind === 'list' || res.kind === 'view') {
        const dataset = res.options.datasets ? res.options.datasets[0] : null
        filterData.value = JSON.stringify(dataset ? dataset.filter : {}, null, 2)
      } else if (res.kind === 'update' || res.kind === 'delete') {
        filterData.value = JSON.stringify(res.options?.filter, null, 2)
      }

      formData.value = {
        kind: res.kind,
        methods: res.methods,
        path: res.path,
        queryParams: [],
        bodyParams: [],
        fieldMapping: fieldMapping ? fieldMapping : {},
        datasets: [],
        middlewares: res.middlewares.length === 0 ? [{
          type: 'main', params: {}
        }] : res.middlewares,
        response: JSON.stringify(res.response),
      }

    }).finally(() => {
      isFormChanged.value = false

      autoSaveTimer.value = setInterval(() => {
        if (isFormChanged.value) {
          onSubmit()
        }
      }, 5000)
    })
  }
})

onUnmounted(() => {
  if (autoSaveTimer.value) {
    clearInterval(autoSaveTimer.value)
  }
})
</script>

<style lang="less" scoped>
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

.form-submit-container {
  display: flex;
  justify-content: center;
  margin: var(--td-comp-margin-xxl) 0 var(--td-comp-margin-xl) 0;
}

.row-gap {
  margin-top: 20px;
}

.side {
  display: flex;
  width: 100%;
  align-items: center;
  justify-content: center;
  margin-top: 50px;

  .line {
    top: 100px;
    width: 2px;
    position: absolute;
    z-index: 1;
    background: linear-gradient(to bottom, var(--td-gray-color-8), transparent 120%);
  }

  .container {
    display: flex;
    align-items: center;
    justify-content: center;
    flex-direction: column;
  }

  .node {
    z-index: 99;
    width: 150px;
    margin-top: 0.6rem;
    margin-bottom: 0.6rem;
    display: flex;
    align-items: center;
    justify-content: center;
    background-color: var(--td-bg-color-container);
    border-radius: var(--td-radius-medium);
    border: 2px solid var(--td-gray-color-8);
    cursor: grab;
    position: relative;

    &:hover {
      .delete-icon {
        display: block !important;
      }
    }

    .delete-icon {
      cursor: pointer;
      display: none;
      position: absolute;
      right: -7px;
      top: -12px;
    }

    .inner {
      padding: 0.6rem;
      justify-content: center;
      align-items: center;
      gap: 5px;

      display: flex;
      width: 100%;
    }
  }

  .main {
    box-shadow: 0 0 10px 0 var(--td-brand-color);
    border: 2px solid var(--td-brand-color);
  }
}

.drag {
  background-color: var(--td-bg-color-container);
  border-radius: var(--td-radius-medium);
  border: 2px dashed var(--td-gray-color-8);
}

.drop {
  background-color: var(--td-bg-color-container);
  border-radius: var(--td-radius-medium);
  border: 2px dashed var(--td-gray-color-8);
}

.drop-placeholder {
  background-color: var(--td-bg-color-container);
  border-radius: var(--td-radius-medium);
  border: 2px dashed var(--td-gray-color-8);
}

.tool {
  display: flex;
  justify-content: center;
  width: 100%;
}

.smooth-dnd-container.vertical > .smooth-dnd-draggable-wrapper {
  overflow: visible !important;
}
</style>
