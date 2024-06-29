<template>
  <t-form
      ref="form"
      class="base-form"
      :data="formData"
      :rules="FORM_RULES"
      label-align="left"
      :label-width="120"
      @reset="onCancel"
      @submit="onSubmit"
  >
    <div class="container">
      <div class="form-basic-item">
        <div class="form-basic-container-title"> {{ mode === 'create' ? '新建接口' : '编辑接口' }}</div>
        <t-row class="row-gap" :gutter="[32, 24]">
          <t-col :span="3">
            <t-form-item label="请求类型" name="methods">
              <t-select v-model="formData.kind" placeholder="请选择请求类型">
                <t-option v-for="kind in ['list','view','create','update', 'delete']" :key="kind" :value="kind"
                          :label="kind.toUpperCase()">
                </t-option>
              </t-select>
            </t-form-item>
          </t-col>
          <t-col :span="3">
            <t-form-item label="请求方式" name="methods">
              <t-select v-model="formData.methods" placeholder="请选择请求方法" multiple>
                <t-option v-for="method in ['GET','POST','PUT','DELETE']" :key="method" :value="method">
                  {{ method }}
                </t-option>
              </t-select>
            </t-form-item>
          </t-col>
          <t-col :span="3">
            <t-form-item label="路径" name="path">
              <t-input v-model="formData.path" :style="{ width: '322px' }" placeholder="请输入请求路径"/>
            </t-form-item>
          </t-col>
        </t-row>
        <t-row class="row-gap" style="margin-top: 15px" :gutter="[32, 24]">

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
</template>

<script setup lang="ts">
import {computed, onMounted, ref} from 'vue'
import {useRoute, useRouter} from "vue-router";
import {FormRule, MessagePlugin, SubmitContext} from "tdesign-vue-next";
import NProgress from "nprogress";
import {createApi, CreateApiReq, getApi, updateApi, UpdateApiReq,} from "@/api/api.ts";

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

const FORM_RULES: Record<string, FormRule[]> = {
  name: [{required: true, message: '请输入项目名', type: 'error'}],
  schemaName: [{required: true, message: '请输入项目ID', type: 'error'}]
};

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
  padding: 0 var(--td-comp-paddingLR-xxl) 80px var(--td-comp-paddingLR-xxl);

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
</style>
