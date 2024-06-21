<template>
  <t-row justify="space-between">
    <div class="operation-container">
      <t-button @click="onCreateProject"> 新建项目</t-button>
    </div>
    <div class="search-input">
      <t-input placeholder="搜索项目" clearable>
        <template #suffix-icon>
          <search-icon size="16px"/>
        </template>
      </t-input>
    </div>
  </t-row>

  <t-table
      :data="projects"
      :columns="COLUMNS"
      row-key="uid"
      vertical-align="top"
      :hover="true"
      :pagination="pagination"
      :loading="isLoading"
      @page-change="pagination = $event; getProjects()"
  >
    <template #createdAt="{row}">
      {{ dayjs(row.createdAt).format('YYYY-MM-DD HH:mm:ss') }}
    </template>
    <template #ops="{row}">
      <t-space>
        <t-link theme="primary" @click="onViewProject(row.uid)">进入项目</t-link>
        <t-link theme="primary" @click="onViewProject(row.uid)">查看</t-link>
        <t-popconfirm content="你确定要删除该项目吗？" @confirm="onDeleteProject">
          <t-link theme="danger"> 删除</t-link>
        </t-popconfirm>
      </t-space>
    </template>
  </t-table>

</template>

<script setup lang="ts">
import {onMounted, ref} from "vue";
import {useRouter} from "vue-router";
import {listProjects, deleteProject, type Project} from "@/api/projects.ts";
import {MessagePlugin, type PaginationProps, PrimaryTableCol, TableRowData} from 'tdesign-vue-next';
import dayjs from 'dayjs'

const router = useRouter()

const COLUMNS: PrimaryTableCol<TableRowData>[] = [
  {colKey: 'name', title: '项目名'},
  {colKey: 'schemaName', title: '项目ID'},
  {colKey: 'createdAt', title: '创建时间'},
  {colKey: 'ops', title: '操作'},
]
const isLoading = ref(false)
const pagination = ref<PaginationProps>({
  total: 0,
})
const projects = ref<Project[]>([])
const getProjects = () => {
  isLoading.value = true

  listProjects().then(res => {
    projects.value = res.projects
    pagination.value.total = res.total
  }).finally(() => {
    isLoading.value = false
  })
}
const onCreateProject = () => {
  router.push({name: 'ProjectCreate'})
}

const onDeleteProject = (projectUID: string) => {
  deleteProject(projectUID).then(() => {
    MessagePlugin.success('删除项目成功')
  }).finally(() => {
    getProjects()
  })
}

const onViewProject = (uid: string) => {
  router.push({name: 'ProjectView', params: {uid}})
}

onMounted(() => {
  getProjects()
})
</script>

<style lang="less" scoped>
.operation-container {
  display: flex;
  align-items: center;
  margin-bottom: var(--td-comp-margin-xxl);
}

.search-input {
  width: 360px;
}
</style>
