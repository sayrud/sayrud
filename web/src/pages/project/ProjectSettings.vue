<template>
  <t-form
      ref="form"
      class="base-form"
      :data="formData"
      :rules="FORM_RULES"
      label-align="top"
      :label-width="100"
      @submit="onSubmit"
  >
    <div class="form-basic-container">
      <div class="form-basic-item">
        <div class="form-basic-container-title"> 项目设置 - {{ project.name }} ({{ project.schemaName }})</div>
        <t-form-item label="项目名" name="name">
          <t-input v-model="formData.name" :style="{ width: '322px' }" placeholder="请输入项目名"/>
        </t-form-item>
        <t-form-item label="项目ID" name="schemaName">
          <t-input disabled v-model="project.schemaName" :style="{ width: '322px' }" placeholder="请输入项目ID"/>
        </t-form-item>
      </div>
    </div>

    <div class="form-submit-container">
      <div class="form-submit-sub">
        <t-space>
          <t-button theme="primary" class="form-submit-confirm" type="submit">确认提交</t-button>
          <t-popconfirm content="你确认要删除该项目吗？该项目下的数据也将被删除且不可恢复！"
                        @confirm="onDelete(project.uid)">
            <t-button theme="danger" class="form-submit-cancel">删除项目</t-button>
          </t-popconfirm>
        </t-space>
      </div>
    </div>
  </t-form>
</template>


<script setup lang="ts">
import {onMounted, ref} from 'vue'
import {getProject, updateProject, UpdateProjectReq, Project, deleteProject} from "@/api/project.ts";
import {FormRule, MessagePlugin, SubmitContext} from 'tdesign-vue-next';
import {useRoute, useRouter} from "vue-router";
import NProgress from "nprogress";
import {useProjectStore} from "@/store";

const route = useRoute()
const router = useRouter()
const projectStore = useProjectStore()
const projectUID = route.params.uid as string
const project = ref<Project>({} as Project)
const FORM_RULES: Record<string, FormRule[]> = {
  name: [{required: true, message: '请输入项目名', type: 'error'}],
};

const formData = ref<UpdateProjectReq>({
  name: '',
})

const fetchProject = () => {
  getProject(projectUID).then((res) => {
    project.value = res
    formData.value = {
      name: res.name,
    }
  }).finally(() => {
    projectStore.setProject(project.value)
  })
}

const onSubmit = (ctx: SubmitContext) => {
  if (ctx.validateResult === true) {
    NProgress.start()

    updateProject(project.value.uid, formData.value).then(() => {
      MessagePlugin.success('项目信息更新成功')
    }).finally(() => {
      fetchProject()
      NProgress.done()
    })
  }
};

const onDelete = (uid: string) => {
  deleteProject(uid).then(() => {
    MessagePlugin.success('项目删除成功')
    projectStore.cleanProject()
    router.push({name: 'Projects'})
  })
}

onMounted(() => {
  fetchProject()
})
</script>

<style scoped>
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
