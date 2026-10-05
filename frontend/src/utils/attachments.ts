import type { Attachment, CellValue } from '../types/bitable.ts'

export const ATTACHMENT_MAX_SIZE = 20 * 1024 * 1024
export const ATTACHMENT_MAX_COUNT = 100

export function attachmentsOf(value: CellValue): Attachment[] {
  return Array.isArray(value)
    ? value.filter((item): item is Attachment => typeof item === 'object' && item !== null && typeof item.uid === 'string')
    : []
}

/** Only the raster formats served inline by storage can be previewed. */
export function isAttachmentImage(file: Attachment): boolean {
  return ['image/jpeg', 'image/png', 'image/gif', 'image/webp', 'image/avif', 'image/bmp'].includes(file.contentType)
}

export function attachmentDownloadURL(file: Attachment): string {
  return `${file.url}${file.url.includes('?') ? '&' : '?'}download=1`
}

export function attachmentSize(size: number): string {
  if (size < 1024) return `${size} B`
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`
  return `${(size / (1024 * 1024)).toFixed(1)} MB`
}
