<script setup lang="ts">
import { Message } from '@arco-design/web-vue'
import { CircleCheck, Eye, EyeOff, GripVertical, PanelRightClose, PanelRightOpen, Share2 } from '@lucide/vue'
import { computed, reactive, ref } from 'vue'

import FieldValueEditor from '@/components/cell/FieldValueEditor.vue'
import FieldTypeIcon from '@/components/field/FieldTypeIcon.vue'
import { useBaseStore } from '@/stores/base'
import type { CellValue, FormFieldConfig, RecordData, SLView } from '@/types/bitable'
import { defaultValueOf, isEmptyValue } from '@/utils/format'

const props = defineProps<{ view: SLView }>()
const store = useBaseStore()

const form = computed(() => props.view.config.form ?? { title: props.view.name, description: '', fields: [] })

/** Form field config in the saved order, with the new fields appended, formula fields can not be filled. */
const fieldConfigs = computed<FormFieldConfig[]>(() => {
  const saved = form.value.fields.filter((c) => store.fields.some((f) => f.uid === c.fieldUID))
  const known = new Set(saved.map((c) => c.fieldUID))
  const added = store.fields
    .filter((f) => !known.has(f.uid) && f.type !== 'formula')
    .map((f) => ({ fieldUID: f.uid, required: false, hidden: false }))
  return [...saved, ...added].filter((c) => store.fields.find((f) => f.uid === c.fieldUID)?.type !== 'formula')
})
const shown = computed(() => fieldConfigs.value.filter((c) => !c.hidden))
const fieldOf = (uid: string) => store.fields.find((f) => f.uid === uid)!

function saveForm(patch: Partial<{ title: string; description: string; fields: FormFieldConfig[] }>) {
  store.updateViewConfig({ form: { ...form.value, fields: fieldConfigs.value, ...patch } })
}
function patchField(uid: string, p: Partial<FormFieldConfig>) {
  saveForm({ fields: fieldConfigs.value.map((c) => (c.fieldUID === uid ? { ...c, ...p } : c)) })
}

// ---- Filling ----
const values = reactive<RecordData>({})
const errors = reactive<Record<string, boolean>>({})
const submitted = ref(false)
const submitting = ref(false)

function reset() {
  for (const k of Object.keys(values)) delete values[k]
  for (const k of Object.keys(errors)) delete errors[k]
  for (const c of shown.value) {
    const v = defaultValueOf(fieldOf(c.fieldUID))
    if (!isEmptyValue(v)) values[c.fieldUID] = v
  }
}
reset()

function setValue(uid: string, v: CellValue) {
  values[uid] = v
  errors[uid] = false
}

async function submit() {
  let ok = true
  for (const c of shown.value) {
    const v = values[c.fieldUID]
    const f = fieldOf(c.fieldUID)
    const empty = f.type === 'checkbox' ? !v : isEmptyValue(v)
    errors[c.fieldUID] = c.required && empty
    if (errors[c.fieldUID]) ok = false
  }
  if (!ok) {
    Message.warning('请填写必填项')
    return
  }
  submitting.value = true
  try {
    const data: RecordData = {}
    for (const c of shown.value) data[c.fieldUID] = values[c.fieldUID] ?? null
    const r = await store.createRecords([data], { withDefaults: false })
    if (r.length) submitted.value = true
  } finally {
    submitting.value = false
  }
}

function again() {
  submitted.value = false
  reset()
}

// ---- Settings panel ----
const settingsOpen = ref(true)
const dragUID = ref('')
const overUID = ref('')
function onDrop() {
  const from = dragUID.value
  const to = overUID.value
  dragUID.value = overUID.value = ''
  if (!from || !to || from === to) return
  const list = [...fieldConfigs.value]
  const fi = list.findIndex((c) => c.fieldUID === from)
  const [item] = list.splice(fi, 1)
  list.splice(
    list.findIndex((c) => c.fieldUID === to),
    0,
    item!,
  )
  saveForm({ fields: list })
}

function share() {
  const url = `${location.origin}/form/${props.view.uid}`
  navigator.clipboard?.writeText(url).catch(() => undefined)
  Message.success('表单链接已复制（Mock）')
}
</script>

<template>
  <div class="form-view">
    <div class="form-scroll">
      <div class="form-actions">
        <a-button size="small" @click="share"><template #icon><Share2 :size="14" /></template>分享表单</a-button>
        <button v-if="store.canEdit" class="icon-btn" :title="settingsOpen ? '收起设置' : '展开设置'" @click="settingsOpen = !settingsOpen">
          <component :is="settingsOpen ? PanelRightClose : PanelRightOpen" :size="16" />
        </button>
      </div>
      <div class="form-card">
        <div class="form-banner" />
        <template v-if="!submitted">
          <div class="form-head">
            <input
              class="form-title"
              :value="form.title"
              placeholder="表单标题"
              :readonly="!store.canEdit"
              @change="(e) => saveForm({ title: (e.target as HTMLInputElement).value })"
            />
            <textarea
              class="form-desc"
              :value="form.description"
              rows="2"
              :placeholder="store.canEdit ? '添加表单描述（选填）' : ''"
              :readonly="!store.canEdit"
              @change="(e) => saveForm({ description: (e.target as HTMLTextAreaElement).value })"
            />
          </div>
          <div v-for="(c, i) in shown" :key="c.fieldUID" class="form-item" :class="{ error: errors[c.fieldUID] }">
            <div class="form-label">
              <span class="form-index">{{ i + 1 }}.</span>
              {{ fieldOf(c.fieldUID).label }}
              <span v-if="c.required" class="required">*</span>
            </div>
            <FieldValueEditor
              :field="fieldOf(c.fieldUID)"
              :value="values[c.fieldUID]"
              @change="(v) => setValue(c.fieldUID, v)"
            />
            <div v-if="errors[c.fieldUID]" class="error-text">此项为必填项</div>
          </div>
          <div v-if="!shown.length" class="no-field">{{ store.canEdit ? '表单中还没有字段，请在右侧设置中开启' : '表单中还没有字段' }}</div>
          <div class="form-submit">
            <a-button type="primary" long size="large" :loading="submitting" :disabled="!store.canEdit" @click="submit">提交</a-button>
            <div v-if="!store.canEdit" class="readonly-tip">你只有查看权限，可预览表单但无法提交</div>
          </div>
        </template>
        <div v-else class="success">
          <CircleCheck :size="56" class="success-icon" />
          <div class="success-title">提交成功</div>
          <div class="text-caption">记录已写入数据表「{{ store.activeTable?.name }}」</div>
          <a-button type="primary" style="margin-top: 24px" @click="again">再填一份</a-button>
        </div>
      </div>
    </div>

    <aside v-if="settingsOpen && store.canEdit" class="settings">
      <div class="settings-title">表单字段</div>
      <div class="settings-tip">拖拽调整顺序，点击眼睛显示或隐藏字段</div>
      <div
        v-for="c in fieldConfigs"
        :key="c.fieldUID"
        class="setting-item"
        :class="{ hidden: c.hidden, over: overUID === c.fieldUID && dragUID !== c.fieldUID }"
        @dragover.prevent="overUID = c.fieldUID"
        @drop.prevent="onDrop"
      >
        <span class="grip" draggable="true" @dragstart="dragUID = c.fieldUID" @dragend="onDrop"><GripVertical :size="14" /></span>
        <FieldTypeIcon :type="fieldOf(c.fieldUID).type" />
        <span class="ellipsis label">{{ fieldOf(c.fieldUID).label }}</span>
        <a-tooltip content="必填" mini>
          <a-checkbox
            :model-value="c.required"
            :disabled="c.hidden"
            @change="(v: boolean | (string | number | boolean)[]) => patchField(c.fieldUID, { required: !!v })"
          >
            必填
          </a-checkbox>
        </a-tooltip>
        <button class="icon-btn sm" @click="patchField(c.fieldUID, { hidden: !c.hidden, required: c.hidden ? c.required : false })">
          <component :is="c.hidden ? EyeOff : Eye" :size="14" />
        </button>
      </div>
      <div class="settings-note">公式字段由系统计算，不出现在表单中。</div>
    </aside>
  </div>
</template>

<style scoped>
.form-view {
  flex: 1;
  min-width: 0;
  display: flex;
  min-height: 0;
  background: var(--bg-base);
}
.form-scroll {
  flex: 1;
  overflow: auto;
  padding: 16px 24px 48px;
}
.form-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  max-width: 680px;
  margin: 0 auto 12px;
}
.form-card {
  max-width: 680px;
  margin: 0 auto;
  background: var(--bg-body);
  border-radius: 12px;
  overflow: hidden;
  box-shadow: var(--shadow-card);
}
.form-banner {
  height: 8px;
  background: linear-gradient(90deg, #3370ff, #14c0a7);
}
.form-head {
  padding: 28px 40px 8px;
}
.form-title {
  width: 100%;
  border: none;
  outline: none;
  font-size: 24px;
  font-weight: 600;
  padding: 4px 0;
  border-bottom: 1px dashed transparent;
}
.form-title:hover,
.form-title:focus {
  border-bottom-color: var(--line-border);
}
.form-desc {
  width: 100%;
  margin-top: 8px;
  border: none;
  outline: none;
  resize: none;
  font: inherit;
  color: var(--text-caption);
  line-height: 1.6;
}
.form-item {
  padding: 12px 40px;
}
.form-label {
  margin-bottom: 8px;
  font-weight: 500;
}
.form-index {
  color: var(--text-placeholder);
  margin-right: 2px;
}
.required {
  color: var(--color-danger);
  margin-left: 2px;
}
.form-item.error :deep(.arco-textarea-wrapper),
.form-item.error :deep(.arco-input-wrapper),
.form-item.error :deep(.select-box),
.form-item.error :deep(.arco-picker) {
  border-color: var(--color-danger);
}
.error-text {
  margin-top: 4px;
  font-size: 12px;
  color: var(--color-danger);
}
.no-field {
  padding: 24px 40px;
  color: var(--text-placeholder);
}
.form-submit {
  padding: 20px 40px 36px;
}
.readonly-tip {
  margin-top: 8px;
  text-align: center;
  font-size: 12px;
  color: var(--text-placeholder);
}
.success {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 72px 24px;
}
.success-icon {
  color: var(--color-success);
}
.success-title {
  margin: 16px 0 8px;
  font-size: 20px;
  font-weight: 600;
}
.settings {
  width: 300px;
  flex: none;
  padding: 16px;
  overflow: auto;
  background: var(--bg-body);
  border-left: 1px solid var(--line-border);
}
.settings-title {
  font-weight: 600;
}
.settings-tip {
  margin: 4px 0 12px;
  font-size: 12px;
  color: var(--text-placeholder);
}
.setting-item {
  display: flex;
  align-items: center;
  gap: 6px;
  height: 36px;
  padding: 0 4px;
  border-radius: 6px;
  border-top: 2px solid transparent;
}
.setting-item:hover {
  background: var(--fill-hover);
}
.setting-item.over {
  border-top-color: var(--color-primary);
}
.setting-item.hidden .label {
  color: var(--text-disabled);
}
.grip {
  display: flex;
  color: var(--text-placeholder);
  cursor: grab;
}
.label {
  flex: 1;
}
.settings-note {
  margin-top: 12px;
  font-size: 12px;
  color: var(--text-placeholder);
}
</style>
