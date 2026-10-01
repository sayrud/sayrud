<script setup lang="ts">
import { Message, Modal } from '@arco-design/web-vue'
import { Check, ChevronDown, Link, UserPlus } from '@lucide/vue'
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'

import { membersApi, projectsApi } from '@/api/bitable'
import { ApiError } from '@/api/client'
import UserAvatar from '@/components/common/UserAvatar.vue'
import { useAuthStore } from '@/stores/auth'
import { useBaseStore } from '@/stores/base'
import type { MemberRole, ProjectMember, UserBrief } from '@/types/bitable'
import { MEMBER_ROLES, ROLE_LABELS } from '@/utils/role'

const visible = defineModel<boolean>('visible', { required: true })

const store = useBaseStore()
const auth = useAuthStore()
const router = useRouter()

const members = ref<ProjectMember[]>([])
const loading = ref(false)

const email = ref('')
const inviteRole = ref<MemberRole>('editor')
const candidate = ref<UserBrief | null>(null)
const lookupError = ref('')
const lookingUp = ref(false)
const inviting = ref(false)

const projectUID = computed(() => store.project?.uid ?? '')
const myID = computed(() => auth.user?.id)
const isEmail = (s: string) => /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(s)
const existing = computed(() => members.value.find((m) => m.user.email === email.value.trim().toLowerCase()))

async function load() {
  if (!projectUID.value) return
  loading.value = true
  try {
    members.value = await membersApi.list(projectUID.value)
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    loading.value = false
  }
}

watch(visible, (v) => {
  if (!v) return
  email.value = ''
  candidate.value = null
  lookupError.value = ''
  load()
})

let lookupTimer: ReturnType<typeof setTimeout> | undefined
let lookupSeq = 0
watch(email, (value) => {
  clearTimeout(lookupTimer)
  candidate.value = null
  lookupError.value = ''
  const v = value.trim()
  lookingUp.value = false
  if (!store.canManage || !isEmail(v)) return
  const seq = ++lookupSeq
  lookingUp.value = true
  lookupTimer = setTimeout(async () => {
    try {
      const user = await membersApi.lookup(projectUID.value, v)
      if (seq === lookupSeq) candidate.value = user
    } catch (e) {
      if (seq === lookupSeq) lookupError.value = e instanceof ApiError && e.status === 404 ? '该邮箱尚未注册，请先让对方注册账号' : String((e as Error).message)
    } finally {
      if (seq === lookupSeq) lookingUp.value = false
    }
  }, 300)
})

async function invite() {
  const v = email.value.trim()
  if (!isEmail(v)) {
    Message.warning('请输入协作者的邮箱')
    return
  }
  inviting.value = true
  try {
    const m = await membersApi.add(projectUID.value, v, inviteRole.value)
    Message.success(`已添加「${m.user.userName}」为${ROLE_LABELS[m.role]}协作者`)
    email.value = ''
    await load()
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
  } finally {
    inviting.value = false
  }
}

async function changeRole(m: ProjectMember, role: MemberRole) {
  if (m.role === role) return
  try {
    await membersApi.update(projectUID.value, m.user.id, role)
    m.role = role
    Message.success(`已将「${m.user.userName}」的权限设为${ROLE_LABELS[role]}`)
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
    load()
  }
}

function remove(m: ProjectMember) {
  const self = m.user.id === myID.value
  Modal.warning({
    title: self ? '退出协作？' : `移除「${m.user.userName}」？`,
    content: self ? '退出后你将无法访问该多维表格，除非被重新添加。' : '移除后对方将无法访问该多维表格。',
    hideCancel: false,
    okText: self ? '退出' : '移除',
    okButtonProps: { status: 'danger' },
    onOk: async () => {
      await membersApi.remove(projectUID.value, m.user.id)
      if (self) {
        visible.value = false
        Message.success('已退出协作')
        router.replace('/')
        return
      }
      Message.success('已移除')
      await load()
    },
  })
}

function transferOwner(m: ProjectMember) {
  Modal.warning({
    title: `将所有权转移给「${m.user.userName}」？`,
    content: '转移后对方成为所有者，你将成为「可管理」协作者，且不能再删除该多维表格。',
    hideCancel: false,
    okText: '转移',
    onBeforeOk: async () => {
      try {
        await projectsApi.transferOwner(projectUID.value, m.user.id)
        store.project = await projectsApi.get(projectUID.value)
      } catch (e) {
        Message.error(e instanceof Error ? e.message : String(e))
        return false
      }
      Message.success(`已将所有权转移给「${m.user.userName}」`)
      await load()
      return true
    },
  })
}

function onMemberAction(m: ProjectMember, value: string) {
  if (value === 'remove') remove(m)
  else if (value === 'transfer') transferOwner(m)
  else changeRole(m, value as MemberRole)
}

/** Owner and the others without the manager role see the label only, managers can change the others, anyone can leave. */
function editable(m: ProjectMember) {
  if (m.role === 'owner') return false
  return m.user.id === myID.value || store.canManage
}

function copyLink() {
  const url = `${location.origin}/base/${projectUID.value}`
  navigator.clipboard?.writeText(url).then(
    () => Message.success('链接已复制，仅协作者可以访问'),
    () => Message.error('复制失败，请手动复制地址栏链接'),
  )
}
</script>

<template>
  <a-modal v-model:visible="visible" :width="520" :footer="false" unmount-on-close>
    <template #title>
      <span class="title ellipsis">分享「{{ store.project?.name }}」</span>
    </template>

    <div v-if="store.canManage" class="invite">
      <div class="invite-row">
        <a-input v-model="email" placeholder="输入协作者的邮箱" allow-clear class="invite-input" @press-enter="invite">
          <template #prefix><UserPlus :size="15" /></template>
          <template #suffix>
            <a-dropdown trigger="click" position="br" :popup-max-height="false" @select="(v) => (inviteRole = v as MemberRole)">
              <a-button type="text" size="mini" class="role-trigger" @click.stop>
                {{ ROLE_LABELS[inviteRole] }}
                <ChevronDown :size="12" class="chevron" />
              </a-button>
              <template #content>
                <a-doption v-for="r in MEMBER_ROLES" :key="r.role" :value="r.role">
                  <div class="role-option">
                    <div class="role-option-main">
                      <span>{{ r.label }}</span>
                      <Check v-if="inviteRole === r.role" :size="14" class="check" />
                    </div>
                    <div class="role-desc">{{ r.description }}</div>
                  </div>
                </a-doption>
              </template>
            </a-dropdown>
          </template>
        </a-input>
        <a-button type="primary" :loading="inviting" :disabled="!!lookupError" @click="invite">邀请</a-button>
      </div>
      <div v-if="lookingUp" class="candidate hint"><a-spin :size="14" /> 查找中…</div>
      <div v-else-if="candidate" class="candidate">
        <UserAvatar :name="candidate.userName" :color="candidate.color" :size="28" />
        <div class="member-text">
          <div class="member-name ellipsis">{{ candidate.userName }}</div>
          <div class="member-email ellipsis">{{ candidate.email }}</div>
        </div>
        <span v-if="existing" class="candidate-tip">已是{{ ROLE_LABELS[existing.role] }}协作者，邀请将更新其权限</span>
      </div>
      <div v-else-if="lookupError" class="candidate hint error">{{ lookupError }}</div>
    </div>
    <div v-else class="invite-tip">你是{{ ROLE_LABELS[store.role ?? 'viewer'] }}协作者，只有可管理权限的协作者才能邀请他人</div>

    <div class="section-title">协作者 · {{ members.length }} 人</div>
    <a-spin :loading="loading" class="list">
      <div v-for="m in members" :key="m.user.id" class="member">
        <UserAvatar :name="m.user.userName" :color="m.user.color" :size="32" />
        <div class="member-text">
          <div class="member-name ellipsis">
            {{ m.user.userName }}<span v-if="m.user.id === myID" class="me">（我）</span>
          </div>
          <div class="member-email ellipsis">{{ m.user.email }}</div>
        </div>
        <a-dropdown
          v-if="editable(m)"
          trigger="click"
          position="br"
          :popup-max-height="false"
          @select="(v) => onMemberAction(m, v as string)"
        >
          <a-button type="text" size="mini" class="role-trigger">
            {{ ROLE_LABELS[m.role] }}
            <ChevronDown :size="12" class="chevron" />
          </a-button>
          <template #content>
            <template v-if="store.canManage && m.user.id !== myID">
              <a-doption v-for="r in MEMBER_ROLES" :key="r.role" :value="r.role">
                <div class="role-option">
                  <div class="role-option-main">
                    <span>{{ r.label }}</span>
                    <Check v-if="m.role === r.role" :size="14" class="check" />
                  </div>
                  <div class="role-desc">{{ r.description }}</div>
                </div>
              </a-doption>
              <a-divider :margin="4" />
            </template>
            <a-doption v-if="store.role === 'owner' && m.user.id !== myID" value="transfer">转移所有权</a-doption>
            <a-doption value="remove" :style="{ color: 'var(--color-danger)' }">
              {{ m.user.id === myID ? '退出协作' : '移除' }}
            </a-doption>
          </template>
        </a-dropdown>
        <span v-else class="role-label">{{ ROLE_LABELS[m.role] }}</span>
      </div>
    </a-spin>

    <div class="footer">
      <span class="footer-tip">仅以上协作者可以通过链接访问</span>
      <a-button @click="copyLink">
        <template #icon><Link :size="14" /></template>
        复制链接
      </a-button>
    </div>
  </a-modal>
</template>

<style scoped>
.title {
  max-width: 420px;
  font-weight: 600;
}
.invite {
  margin-bottom: 20px;
}
.invite-row {
  display: flex;
  gap: 8px;
}
.invite-input {
  flex: 1;
}
/* Arco text buttons are primary colored, the role triggers stay neutral like plain labels. */
.role-trigger.arco-btn {
  height: 26px;
  padding: 0 6px;
  color: var(--text-caption);
  font-size: 13px;
}
.role-trigger.arco-btn:hover {
  background: var(--fill-hover);
  color: var(--text-title);
}
.chevron {
  margin-left: 2px;
}
/* Arco options have a 36px line height, reset it so the two lines stay compact and centered by the padding. */
.role-option {
  width: 220px;
  padding: 8px 0;
  line-height: normal;
}
.role-option-main {
  display: flex;
  align-items: center;
  justify-content: space-between;
  line-height: 22px;
}
.role-desc {
  margin-top: 2px;
  font-size: 12px;
  line-height: 18px;
  color: var(--text-placeholder);
}
.check {
  color: var(--color-primary);
}
.candidate {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 8px;
  padding: 8px 10px;
  border-radius: 6px;
  background: var(--bg-base);
}
.candidate.hint {
  gap: 6px;
  color: var(--text-caption);
  font-size: 13px;
}
.candidate.error {
  color: var(--color-danger);
}
.candidate-tip {
  margin-left: auto;
  font-size: 12px;
  color: var(--color-warning);
}
.invite-tip {
  margin-bottom: 20px;
  padding: 8px 12px;
  border-radius: 6px;
  background: var(--bg-base);
  color: var(--text-caption);
  font-size: 13px;
}
.section-title {
  margin-bottom: 8px;
  font-size: 13px;
  font-weight: 500;
  color: var(--text-caption);
}
.list {
  display: block;
  max-height: 320px;
  overflow: auto;
}
.member {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 4px;
  border-radius: 6px;
}
.member:hover {
  background: var(--fill-hover);
}
.member-text {
  flex: 1;
  min-width: 0;
}
.member-name {
  color: var(--text-title);
}
.me {
  color: var(--text-placeholder);
}
.member-email {
  font-size: 12px;
  color: var(--text-placeholder);
}
.role-label {
  padding: 0 6px;
  font-size: 13px;
  color: var(--text-placeholder);
}
.footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--line-border);
}
.footer-tip {
  font-size: 12px;
  color: var(--text-placeholder);
}
</style>
