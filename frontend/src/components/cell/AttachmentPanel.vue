<script setup lang="ts">
import { Message } from '@arco-design/web-vue'
import { Download, Eye, File as FileIcon, Move, Plus, X } from '@lucide/vue'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { attachmentsApi } from '@/api/bitable'
import StateIllustration from '@/components/common/StateIllustration.vue'
import { useBaseStore } from '@/stores/base'
import type { Attachment, CellValue, SLField } from '@/types/bitable'
import { ATTACHMENT_MAX_COUNT, ATTACHMENT_MAX_SIZE, attachmentDownloadURL as downloadURL, attachmentsOf, attachmentSize, isAttachmentImage } from '@/utils/attachments'

const props = defineProps<{ field: SLField; value: CellValue; recordUID?: string; readonly?: boolean; autofocus?: boolean }>()
const emit = defineEmits<{ change: [value: CellValue]; busy: [value: boolean]; preview: [value: boolean] }>()
const { t } = useI18n()
const store = useBaseStore()
const files = computed(() => attachmentsOf(props.value))
const editable = computed(() => !props.readonly && store.canEdit)
const uploading = ref(false)
const progress = ref(0)
const panel = ref<HTMLElement>()
const dragging = ref(false)
const reorderUID = ref<string>()
const dropTargetUID = ref<string>()
const previewFile = ref<Attachment>()
const controller = new AbortController()
const selection: File[] = []
let mounted = true

onMounted(() => {
  if (props.autofocus) panel.value?.focus({ preventScroll: true })
})

onBeforeUnmount(() => {
  if (uploading.value) emit('busy', false)
  mounted = false
  // Record uploads can finish after closing their panel. Unsaved form drafts belong to the form.
  if (!props.recordUID) controller.abort()
})

function remove(uid: string) {
  if (!editable.value || uploading.value) return
  const next = files.value.filter((file) => file.uid !== uid)
  emit('change', next.length ? next : null)
}

function setPreview(file?: Attachment) {
  previewFile.value = file
  emit('preview', !!file)
}

function moveAttachment(uid: string, targetUID: string) {
  if (!editable.value || uploading.value) return
  const next = [...files.value]
  const from = next.findIndex((file) => file.uid === uid)
  const to = next.findIndex((file) => file.uid === targetUID)
  if (from < 0 || to < 0 || from === to) return
  next.splice(to, 0, next.splice(from, 1)[0]!)
  emit('change', next)
}

function moveBy(uid: string, offset: number) {
  const index = files.value.findIndex((file) => file.uid === uid)
  const target = files.value[index + offset]
  if (target) moveAttachment(uid, target.uid)
}

function endReorder() {
  reorderUID.value = dropTargetUID.value = undefined
}

function startReorder(event: DragEvent, uid: string) {
  if (!editable.value || uploading.value) {
    event.preventDefault()
    return
  }
  reorderUID.value = uid
  event.dataTransfer?.setData('application/x-sayrud-attachment', uid)
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move'
    const card = (event.currentTarget as HTMLElement).closest('.attachment-card')
    if (card) event.dataTransfer.setDragImage(card, card.clientWidth / 2, card.clientHeight / 2)
  }
}

function onCardDragOver(event: DragEvent, uid: string) {
  if (!reorderUID.value || !editable.value || uploading.value) return
  event.preventDefault()
  event.stopPropagation()
  dropTargetUID.value = uid === reorderUID.value ? undefined : uid
  if (event.dataTransfer) event.dataTransfer.dropEffect = 'move'
}

function onCardDrop(event: DragEvent, uid: string) {
  const from = reorderUID.value
  if (!from) return
  event.preventDefault()
  event.stopPropagation()
  endReorder()
  moveAttachment(from, uid)
}

function beforeUpload(file: File) {
  selection.push(file)
  // Arco calls this hook per file; collect one selection for batch validation and sequential uploads.
  if (selection.length === 1) queueMicrotask(() => void uploadFiles(selection.splice(0)))
  return false
}

function onPaste(event: ClipboardEvent) {
  const selected = Array.from(event.clipboardData?.files ?? [])
  if (!selected.length) return
  event.preventDefault()
  event.stopPropagation()
  return uploadFiles(selected)
}

function onDragOver(event: DragEvent) {
  if (!event.dataTransfer?.types.includes('Files')) return
  event.preventDefault()
  event.stopPropagation()
  dragging.value = editable.value && !uploading.value && files.value.length < ATTACHMENT_MAX_COUNT
  event.dataTransfer.dropEffect = dragging.value ? 'copy' : 'none'
}

function onDrop(event: DragEvent) {
  dragging.value = false
  const selected = Array.from(event.dataTransfer?.files ?? [])
  if (!selected.length) return
  event.preventDefault()
  event.stopPropagation()
  return uploadFiles(selected)
}

async function uploadFiles(selected: File[]) {
  if (!mounted || !selected.length || !editable.value || uploading.value) return
  if (files.value.length + selected.length > ATTACHMENT_MAX_COUNT) {
    Message.warning(t('cell.attachmentLimitReached'))
    return
  }
  if (selected.some((file) => file.size > ATTACHMENT_MAX_SIZE)) {
    Message.warning(t('cell.attachmentTooLarge'))
    return
  }
  const projectUID = store.project?.uid
  if (!projectUID) return
  const field = props.field
  const recordUID = props.recordUID
  let draft = [...files.value]
  uploading.value = true
  progress.value = 0
  emit('busy', true)
  try {
    for (let i = 0; i < selected.length; i++) {
      const file = await attachmentsApi.upload(projectUID, field, selected[i]!, {
        signal: controller.signal,
        onUploadProgress: ({ loaded, total }) => {
          progress.value = Math.round(((i + (total ? loaded / total : 0)) / selected.length) * 100)
        },
      })
      if (recordUID) {
        // Merge with the latest cell to preserve changes made while the file was uploading.
        const record = store.recordMap.get(recordUID)
        if (store.project?.uid !== projectUID || !record || !store.canEdit || store.fields.find((f) => f.uid === field.uid)?.type !== 'attachment') break
        const current = attachmentsOf(record.data[field.uid])
        if (current.length >= ATTACHMENT_MAX_COUNT) {
          Message.warning(t('cell.attachmentLimitReached'))
          break
        }
        await store.updateCell(recordUID, field.uid, [...current, file])
      } else {
        draft = [...draft, file]
        emit('change', draft)
      }
    }
  } catch (error) {
    if (!controller.signal.aborted) Message.error(error instanceof Error ? error.message : t('cell.attachmentUploadFailed'))
  } finally {
    uploading.value = false
    if (mounted) emit('busy', false)
  }
}

</script>

<template>
  <div ref="panel" class="attachment-panel" tabindex="-1" role="group" :aria-label="field.label" :aria-busy="uploading" @mousedown.stop @keydown.stop @paste="onPaste" @dragover="onDragOver" @dragleave="dragging = false" @drop="onDrop">
    <div v-if="files.length" class="attachments">
      <a-card v-for="file in files" :key="file.uid" class="attachment-card" :class="{ 'image-card': isAttachmentImage(file), 'is-dragging': reorderUID === file.uid, 'drop-target': dropTargetUID === file.uid }" :data-attachment="file.uid" :body-style="{ padding: 0 }" @dragover="onCardDragOver($event, file.uid)" @drop="onCardDrop($event, file.uid)">
        <a-image v-if="isAttachmentImage(file)" :src="file.url" :alt="file.name" width="100%" height="108" fit="cover" class="image" :preview="false" :draggable="false" @click="setPreview(file)" />
        <a-button v-else type="text" class="file-preview" :href="downloadURL(file)" :title="file.name" :download="file.name">
          <FileIcon :size="32" />
          <span>{{ file.name.split('.').pop()?.toUpperCase() }}</span>
          <span class="file-size">{{ attachmentSize(file.size) }}</span>
        </a-button>
        <div class="attachment-overlay" @click="isAttachmentImage(file) && setPreview(file)">
          <span class="file-name" :title="file.name">{{ file.name }}</span>
        </div>
        <div class="file-actions">
          <a-button v-if="isAttachmentImage(file)" type="text" size="mini" :title="t('cell.previewAttachment')" :aria-label="t('cell.previewAttachment')" @click.stop="setPreview(file)"><template #icon><Eye :size="18" /></template></a-button>
          <a-button type="text" size="mini" :href="downloadURL(file)" :download="file.name" :title="t('cell.downloadAttachment')" :aria-label="t('cell.downloadAttachment')" @click.stop><template #icon><Download :size="18" /></template></a-button>
          <span v-if="editable" role="button" class="reorder-handle" :tabindex="uploading ? -1 : 0" :aria-disabled="uploading" :draggable="!uploading" :title="t('cell.reorderAttachment')" :aria-label="`${t('cell.reorderAttachment')} ${file.name}`" @click.stop @dragstart.stop="startReorder($event, file.uid)" @dragend.stop="endReorder" @keydown.left.prevent.stop="moveBy(file.uid, -1)" @keydown.right.prevent.stop="moveBy(file.uid, 1)"><Move :size="18" /></span>
        </div>
        <a-button v-if="editable" type="text" size="mini" class="remove-attachment" :disabled="uploading" :title="t('cell.removeAttachment')" :aria-label="t('cell.removeAttachment')" @click.stop="remove(file.uid)"><template #icon><X :size="14" /></template></a-button>
      </a-card>
    </div>
    <a-empty v-else-if="!editable" class="empty" :description="t('cell.noAttachments')"><template #image><StateIllustration name="no-files" :width="88" /></template></a-empty>

    <a-upload v-if="editable" class="attachment-upload" multiple draggable :auto-upload="false" :show-file-list="false" :disabled="uploading || files.length >= ATTACHMENT_MAX_COUNT" :on-before-upload="beforeUpload" @drop.stop="dragging = false">
      <template #upload-button>
        <a-empty v-if="!files.length" class="drop-zone" :class="{ dragging }" tabindex="0" :description="t('cell.attachmentDropHint')"><template #image><StateIllustration name="no-files" :width="72" /></template></a-empty>
        <a-button type="text" long class="add-files" :loading="uploading" :disabled="files.length >= ATTACHMENT_MAX_COUNT">
          <template #icon><Plus :size="16" /></template>
          {{ uploading ? `${t('cell.attachmentUploading')} ${progress}%` : t('cell.addLocalFiles') }}
        </a-button>
      </template>
    </a-upload>
    <a-image-preview :src="previewFile?.url" :visible="!!previewFile" @close="setPreview()" />
  </div>
</template>

<style scoped>
.attachment-panel { padding: 12px; min-width: 0; }
.attachment-panel:focus { outline: none; }
.attachment-upload { display: block; }
.drop-zone { display: flex; align-items: center; justify-content: center; gap: 12px; min-height: 108px; padding: 12px; border: 1px dashed var(--line-border); border-radius: 5px; background: var(--bg-base); cursor: pointer; }
.drop-zone :deep(.arco-empty-image) { flex: none; margin: 0; }
.drop-zone :deep(.arco-empty-description) { color: var(--text-caption); }
.drop-zone.dragging, .drop-zone:focus-visible { border-color: var(--color-primary); background: var(--bg-primary-soft); outline: none; }
.attachments { display: grid; grid-template-columns: repeat(auto-fill, minmax(126px, 1fr)); align-items: start; gap: 10px; max-height: 330px; overflow: auto; }
.attachment-card { position: relative; overflow: hidden; min-width: 0; border: 1px solid var(--line-border); border-radius: 6px; background: var(--bg-body); }
.attachment-card.is-dragging { opacity: 0.5; }
.attachment-card.drop-target { outline: 2px solid var(--color-primary); outline-offset: -2px; }
.image { display: block; }
.image-card .image, .image-card .attachment-overlay { cursor: zoom-in; }
.attachment-overlay { position: absolute; inset: 0; padding: 8px 32px 8px 8px; background: rgba(31, 35, 41, 0.4); color: #fff; }
.file-name { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 12px; }
.attachment-overlay, .file-actions, .remove-attachment { opacity: 0; pointer-events: none; }
.attachment-card:hover .attachment-overlay, .attachment-card:hover .file-actions, .attachment-card:hover .remove-attachment,
.attachment-card:focus-within .attachment-overlay, .attachment-card:focus-within .file-actions, .attachment-card:focus-within .remove-attachment { opacity: 1; pointer-events: auto; }
.file-preview.arco-btn { display: flex; flex-direction: column; gap: 8px; width: 100%; height: 108px; padding: 0; background: var(--fill-hover); color: var(--text-caption); }
.file-preview span { font-size: 11px; max-width: 100%; overflow: hidden; text-overflow: ellipsis; }
.file-size { color: var(--text-placeholder); font-size: 11px; }
.attachment-card:hover .file-preview :deep(*), .attachment-card:focus-within .file-preview :deep(*) { opacity: 0; }
.file-actions { position: absolute; top: 50%; left: 50%; display: flex; gap: 12px; transform: translate(-50%, -50%); }
.file-actions :deep(.arco-btn), .reorder-handle { width: 26px; height: 26px; padding: 0; color: #fff; }
.remove-attachment.arco-btn { position: absolute; top: 5px; right: 5px; width: 22px; height: 22px; padding: 0; border-radius: 50%; background: rgba(255, 255, 255, 0.5); color: #fff; }
.file-actions :deep(.arco-btn:hover), .reorder-handle:hover { background: rgba(255, 255, 255, 0.2); }
.reorder-handle { display: flex; align-items: center; justify-content: center; border-radius: 4px; cursor: grab; }
.reorder-handle:focus-visible { outline: 2px solid var(--color-primary); outline-offset: -2px; }
.reorder-handle[aria-disabled="true"] { opacity: 0.5; cursor: default; }
.reorder-handle:active { cursor: grabbing; }
.add-files.arco-btn { height: 42px; margin-top: 10px; border-radius: 5px; color: var(--text-title); }
.add-files:hover:enabled { background: var(--fill-hover); }
.add-files:disabled { color: var(--text-disabled); cursor: default; }
.empty { padding: 8px; font-size: 12px; }
</style>
