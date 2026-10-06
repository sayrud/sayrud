<script setup lang="ts">
import { Message, Modal } from '@arco-design/web-vue'
import { Check, ChevronDown, Link, UserPlus } from '@lucide/vue'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'

import { membersApi, projectsApi } from '@/api/bitable'
import { ApiError } from '@/api/client'
import UserAvatar from '@/components/common/UserAvatar.vue'
import LinkSharePanel from '@/components/base/LinkSharePanel.vue'
import { useAuthStore } from '@/stores/auth'
import { useBaseStore } from '@/stores/base'
import type { MemberRole, ProjectMember, UserBrief } from '@/types/bitable'
import { MEMBER_ROLES, ROLE_LABELS } from '@/utils/role'

const { t } = useI18n()

const visible = defineModel<boolean>('visible', { required: true })

const store = useBaseStore()
const auth = useAuthStore()
const router = useRouter()

const members = ref<ProjectMember[]>([])
const loading = ref(false)
const publicLink = ref('')
const linkSharePanel = ref<InstanceType<typeof LinkSharePanel> | null>(null)

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
      if (seq === lookupSeq) lookupError.value = e instanceof ApiError && e.status === 404 ? t('share.notRegistered') : String((e as Error).message)
    } finally {
      if (seq === lookupSeq) lookingUp.value = false
    }
  }, 300)
})

async function invite() {
  const v = email.value.trim()
  if (!isEmail(v)) {
    Message.warning(t('share.emailRequired'))
    return
  }
  inviting.value = true
  try {
    const m = await membersApi.add(projectUID.value, v, inviteRole.value)
    Message.success(t('share.added', { name: m.user.userName, role: ROLE_LABELS[m.role] }))
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
    Message.success(t('share.roleChanged', { name: m.user.userName, role: ROLE_LABELS[role] }))
  } catch (e) {
    Message.error(e instanceof Error ? e.message : String(e))
    load()
  }
}

function remove(m: ProjectMember) {
  const self = m.user.id === myID.value
  Modal.warning({
    title: self ? t('share.leaveTitle') : t('share.removeTitle', { name: m.user.userName }),
    content: self ? t('home.leaveContent') : t('share.removeContent'),
    hideCancel: false,
    okText: self ? t('home.leaveOk') : t('share.remove'),
    okButtonProps: { status: 'danger' },
    onOk: async () => {
      await membersApi.remove(projectUID.value, m.user.id)
      if (self) {
        visible.value = false
        Message.success(t('home.left'))
        router.replace('/')
        return
      }
      Message.success(t('share.removed'))
      await load()
    },
  })
}

function transferOwner(m: ProjectMember) {
  Modal.warning({
    title: t('share.transferTitle', { name: m.user.userName }),
    content: t('share.transferContent'),
    hideCancel: false,
    okText: t('admin.projects.transfer'),
    onBeforeOk: async () => {
      try {
        await projectsApi.transferOwner(projectUID.value, m.user.id)
        store.project = await projectsApi.get(projectUID.value)
      } catch (e) {
        Message.error(e instanceof Error ? e.message : String(e))
        return false
      }
      Message.success(t('share.transferred', { name: m.user.userName }))
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
  if (publicLink.value && linkSharePanel.value) {
    void linkSharePanel.value.copyLink()
    return
  }

  const path = router.resolve({ name: 'base', params: {
    projectUID: projectUID.value, tableUID: store.activeTableUID || undefined, viewUID: store.activeViewUID || undefined,
  } }).href
  const url = new URL(path, location.origin).href
  navigator.clipboard?.writeText(url).then(
    () => Message.success(t('home.linkCopied')),
    () => Message.error(t('share.copyFailed')),
  )
}
</script>

<template>
  <a-modal v-model:visible="visible" :width="520" :footer="false" unmount-on-close>
    <template #title>
      <span class="title ellipsis">{{ t('share.title', { name: store.project?.name ?? '' }) }}</span>
    </template>

    <div v-if="store.canManage" class="invite">
      <div class="invite-row">
        <a-input v-model="email" :placeholder="t('share.emailPlaceholder')" allow-clear class="invite-input" @press-enter="invite">
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
        <a-button type="primary" :loading="inviting" :disabled="!!lookupError" @click="invite">{{ t('share.invite') }}</a-button>
      </div>
      <div v-if="lookingUp" class="candidate hint"><a-spin :size="14" /> {{ t('share.lookingUp') }}</div>
      <div v-else-if="candidate" class="candidate">
        <UserAvatar :name="candidate.userName" :color="candidate.color" :avatar-url="candidate.avatarUrl" :size="28" />
        <div class="member-text">
          <div class="member-name ellipsis">{{ candidate.userName }}</div>
          <div class="member-email ellipsis">{{ candidate.email }}</div>
        </div>
        <span v-if="existing" class="candidate-tip">{{ t('share.existing', { role: ROLE_LABELS[existing.role] }) }}</span>
      </div>
      <div v-else-if="lookupError" class="candidate hint error">{{ lookupError }}</div>
    </div>
    <div v-else class="invite-tip">{{ t('share.cannotInvite', { role: ROLE_LABELS[store.role ?? 'viewer'] }) }}</div>

    <div class="section-title">{{ t('share.members', { n: members.length }, members.length) }}</div>
    <a-spin :loading="loading" class="list">
      <div v-for="m in members" :key="m.user.id" class="member">
        <UserAvatar :name="m.user.userName" :color="m.user.color" :avatar-url="m.user.avatarUrl" :size="32" />
        <div class="member-text">
          <div class="member-name ellipsis">
            {{ m.user.userName }}<span v-if="m.user.id === myID" class="me">{{ t('share.me') }}</span>
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
            <a-doption v-if="store.role === 'owner' && m.user.id !== myID" value="transfer">{{ t('share.transfer') }}</a-doption>
            <a-doption value="remove" :style="{ color: 'var(--color-danger)' }">
              {{ m.user.id === myID ? t('home.leave') : t('share.remove') }}
            </a-doption>
          </template>
        </a-dropdown>
        <span v-else class="role-label">{{ ROLE_LABELS[m.role] }}</span>
      </div>
    </a-spin>

    <LinkSharePanel ref="linkSharePanel" :visible="visible" @change="(url) => (publicLink = url)" />

    <div class="footer">
      <a-button @click="copyLink">
        <template #icon><Link :size="14" /></template>
        {{ linkSharePanel?.hasPassword ? t('share.copyLinkPassword') : t('home.copyLink') }}
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
  justify-content: flex-end;
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--line-border);
}
</style>
