<template>
  <t-row :gutter="16">
    <t-col :span="9">
      <t-form
          ref="form"
          class="base-form"
          :data="formData"
          :rules="FORM_RULES"
          label-align="right"
          :label-width="70"
          @reset="onCancel"
          @submit="onSubmit"
      >
        <div class="container">
          <div class="form-basic-item">
            <div class="form-basic-container-title"> {{ mode === 'create' ? '新建接口' : '编辑接口' }}</div>
            <t-row class="row-gap" :gutter="[32, 24]">
              <t-col :span="4">
                <t-form-item label="接口类型" name="methods">
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
                      <t-button theme="default" variant="outline" @click="onAddRow">
                        <t-icon name="add"/>
                      </t-button>
                    </div>
                    <t-table
                        ref="paramsTableRef"
                        row-key="key"
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
                      <template #ops="{row, rowIndex}">
                        <t-space>
                          <t-link v-if="!requestParamsEditableRowKeys.includes(row.key)" theme="primary" hover="color"
                                  @click="onEditRow(row.key)">编辑
                          </t-link>
                          <t-link v-if="requestParamsEditableRowKeys.includes(row.key)" theme="primary" hover="color"
                                  @click="onSaveRow(rowIndex, row.key)">保存
                          </t-link>
                          <t-link v-if="requestParamsEditableRowKeys.includes(row.key)" theme="primary" hover="color"
                                  @click="onCancelRow(rowIndex)">取消
                          </t-link>
                          <t-popconfirm theme="danger" content="你确定要删除该参数吗？"
                                        @confirm="onDeleteRow(rowIndex)">
                            <t-link theme="danger" hover="color"> 删除</t-link>
                          </t-popconfirm>
                        </t-space>
                      </template>
                    </t-table>
                  </t-space>
                </t-form-item>
              </t-col>
            </t-row>
            <t-row class="row-gap" :gutter="[32, 24]">
              <t-col :span="12">
                <t-form-item label="响应格式">
                </t-form-item>
              </t-col>
            </t-row>
          </div>
        </div>

        <div class="form-submit-container">
          <div class="form-submit-sub">
            <t-space>
              <t-button theme="primary" class="form-submit-confirm" type="submit">确认提交</t-button>
              <t-button type="reset" class="form-submit-cancel" theme="default" variant="base">取消</t-button>
            </t-space>
          </div>
        </div>
      </t-form>
    </t-col>
    <t-col :span="3">
      <div class="side">
        <Container orientation="vertical"
                   class="container"
                   @drop="onDropMiddlewares">
          <Draggable v-for="item in middlewares" :key="item.id" :class="['node',item.type === 'main'? 'main': '']">
            <div class="inner">{{ item.name }}</div>
          </Draggable>
        </Container>
        <div class="line" :style="{height: `${middlewares.length*70}px`}"></div>
      </div>
      <div class="tool">
        <t-button shape="circle" theme="primary">
          <template #icon>
            <add-icon/>
          </template>
        </t-button>
      </div>
    </t-col>
  </t-row>
</template>

<script setup lang="ts">
import {onMounted, ref, computed} from 'vue'
import {useRoute, useRouter} from "vue-router";
import {
  FormRule,
  MessagePlugin,
  type PrimaryTableCol,
  SubmitContext,
  TableRowData,
  type TableProps, Input, Select, Switch,
  type TableInstanceFunctions,
} from "tdesign-vue-next";
import NProgress from "nprogress";
import {
  createApi,
  CreateApiReq,
  getApi,
  Param, ParamKindLabels, ParamKindOptions, ParamMixin, ParamType,
  ParamTypeLabels,
  ParamTypeOptions,
  updateApi,
  UpdateApiReq
} from "@/api/api.ts";
import {Container, Draggable} from "vue3-smooth-dnd";
import {AddIcon} from 'tdesign-icons-vue-next';
import {allTables, type Table} from '@/api/schemalessTable'

const route = useRoute()
const router = useRouter()
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
  schemaName: [{required: true, message: '请输入项目ID', type: 'error'}]
};

// Request params table
const paramsTableRef = ref<TableInstanceFunctions>()
const paramsColumns = computed<TableProps['columns']>(() => [
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
      rules: [{required: true, message: '不能为空'}],
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
    colKey: 'customValidators', title: '自定义校验规则', align: 'center', edit: {
      component: Input,
      props: {clearable: true, size: 'small'},
      showEditIcon: false
    }
  },
  {colKey: 'ops', title: '操作', width: 150, align: 'center',},
])
const requestParamsEditMap: Record<number, TableRowData> = {}
const requestParams = ref<ParamMixin[]>([{
  kind: 'query',
  key: 'name',
  label: '名称',
  type: 'string',
  required: false,
  customValidators: [],
}])
const requestParamsEditableRowKeys = ref<string[]>([])

const onAddRow = () => {
  requestParams.value.push({
    kind: 'query',
    key: '',
    label: '',
    type: 'string',
    required: false,
    customValidators: [],
  })
}
const onEditRow = (key: string) => {
  if (!requestParamsEditableRowKeys.value.includes(key)) {
    requestParamsEditableRowKeys.value.push(key)
    paramsTableRef.value?.clearValidateData()
  }
}

const currentSaveKey = ref<number>()
const onSaveRow = (rowIndex: number, key: string) => {
  currentSaveKey.value = rowIndex
  paramsTableRef.value?.validateRowData(key).then((params) => {
    if (params.result.length) {
      const r = params.result[0]
      MessagePlugin.error(`${r.col.title} ${r.errorList[0].message}`)
      return
    }

    // Invoked by the table component.
    if (params.trigger === 'parent' && !params.result.length) {
      const current = requestParamsEditMap[rowIndex]
      if (current) {
        requestParams.value.splice(rowIndex, 1, current.editedRow)
      }
      requestParamsEditableRowKeys.value.splice(rowIndex, 1)
    }
  })
}

const onCancelRow = (rowIndex: number) => {
  requestParamsEditableRowKeys.value.splice(rowIndex, 1)
}

const onDeleteRow = (key: number) => {
  requestParams.value.splice(key, 1)
}
const onParamsRowEdit: TableProps['onRowEdit'] = (params) => {
  const {row, col, value, rowIndex} = params;
  const oldRowData = requestParamsEditMap[rowIndex]?.editedRow || row;
  const editedRow = {
    ...oldRowData,
    [col.colKey as string]: value,
  }
  requestParamsEditMap[rowIndex] = {
    ...params,
    editedRow,
  }
}

const onParamsRowValidate = () => {

}

const onParamsValidate = () => {

}

const formData = ref<CreateApiReq | UpdateApiReq>({
  kind: 'list',
  methods: [],
  path: '',
  queryParams: [{
    key: '',
    label: '',
    type: 'string',
    required: false,
    customValidators: [],
  }],
  bodyParams: [],
  datasets: [],
  response: {},
})

const middlewares = ref([
  {name: 'API', type: 'main'},
])

const onDropMiddlewares = (dropResult) => {
  const {removedIndex, addedIndex, payload} = dropResult;

  if (removedIndex === null && addedIndex === null) {
    return
  }
  const result = [...middlewares.value];
  let itemToAdd = payload;

  if (removedIndex !== null) {
    itemToAdd = result.splice(removedIndex, 1)[0];
  }
  if (addedIndex !== null) {
    result.splice(addedIndex, 0, itemToAdd);
  }
  middlewares.value = result
}

const onSubmit = (ctx: SubmitContext) => {
  if (ctx.validateResult === true) {
    NProgress.start()

    if (mode.value === 'create') {
      createApi(projectUID, formData.value as CreateApiReq).then(() => {
        MessagePlugin.success('新建接口成功')
        router.push({name: 'SchemalessApis', params: {uid: projectUID}})
      }).finally(() => {
        NProgress.done()
      })

    } else {
      updateApi(projectUID, apiUID.value, formData.value as UpdateApiReq).then(() => {
        MessagePlugin.success('更新接口成功')
      }).finally(() => {
        NProgress.done()
      })
    }
  }
};

const onCancel = () => {
  router.push({name: 'SchemalessApis', params: {uid: projectUID}})
}

onMounted(() => {
  allTables(projectUID).then(res => {
    tables.value = res
  })

  if (mode.value === 'update') {
    getApi(projectUID, apiUID.value).then(res => {
      formData.value = {
        kind: res.kind,
        methods: res.methods,
        path: res.path,
        queryParams: res.queryParams,
        bodyParams: res.bodyParams,
        datasets: res.options['datasets'],
        response: res.response,
      }
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
    }
  }
}

.form-submit-container {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  padding-top: var(--td-comp-paddingLR-xl);
  padding-bottom: var(--td-comp-paddingLR-xl);
  background-color: var(--td-bg-color-secondarycontainer);
  border-bottom-left-radius: var(--td-radius-medium);
  border-bottom-right-radius: var(--td-radius-medium);
  border-top: 1px solid var(--td-component-stroke);

  .form-submit-sub {
    width: 676px;
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
}

.row-gap {
  margin-top: 20px;
}

.side {
  display: flex;
  width: 100%;
  align-items: center;
  justify-content: center;

  .line {
    top: 0;
    width: 2px;
    background-color: var(--td-gray-color-8);
    position: absolute;
    z-index: 1;
  }

  .container {
    display: flex;
    align-items: center;
    justify-content: center;
    flex-direction: column;
    gap: 30px;
  }

  .node {
    z-index: 99;
    width: 150px;
    height: 50px;
    display: flex;
    align-items: center;
    justify-content: center;
    background-color: var(--td-bg-color-container);
    border-radius: var(--td-radius-medium);
    border: 2px solid var(--td-gray-color-8);
    cursor: grab;

    .inner {
      justify-content: center;
      display: flex;
      width: 150px;
    }
  }

  .main {
    box-shadow: 0 0 10px 0 var(--td-brand-color);
    border: 2px solid var(--td-brand-color);
  }
}

.tool {
  display: flex;
  justify-content: center;
  width: 100%;
}
</style>
