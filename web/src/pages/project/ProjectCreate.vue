<template>
  <t-form
      ref="form"
      class="base-form"
      :data="formData"
      :rules="FORM_RULES"
      label-align="top"
      :label-width="100"
      @reset="onCancel"
      @submit="onSubmit"
  >
    <div class="form-basic-container">
      <div class="form-basic-item">
        <div class="form-basic-container-title"> 新建项目</div>
        <t-form-item label="项目名" name="name">
          <t-input v-model="formData.name" :style="{ width: '322px' }" placeholder="请输入项目名"/>
        </t-form-item>
        <t-form-item label="项目ID" name="schemaName">
          <t-input v-model="formData.schemaName" :style="{ width: '322px' }" placeholder="请输入项目ID"/>
        </t-form-item>
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
import {ref} from 'vue'
import {createProject, type CreateProjectReq} from "@/api/projects";
import type {FormRule, SubmitContext} from 'tdesign-vue-next';
import {useRouter} from "vue-router";
import NProgress from "nprogress";

const router = useRouter()
const FORM_RULES: Record<string, FormRule[]> = {
  name: [{required: true, message: '请输入项目名', type: 'error'}],
  schemaName: [{required: true, message: '请输入项目ID', type: 'error'}]
};

const formData = ref<CreateProjectReq>({
  name: '',
  schemaName: '',
})

const onSubmit = (ctx: SubmitContext) => {
  if (ctx.validateResult === true) {
    NProgress.start()

    createProject(formData.value).then(() => {
      router.push({name: 'Projects'})
    }).finally(() => {
      NProgress.done()
    })
  }
};

const onCancel = () => {
  router.push({name: 'Projects'})
}
</script>

<style scoped>
.form-basic-container {
  display: flex;
  align-items: center;
  justify-content: center;
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
    width: 676px;

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
</style>
