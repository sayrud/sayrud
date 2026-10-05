import assert from 'node:assert/strict'
import test from 'node:test'

import type { Attachment } from '../src/types/bitable.ts'
import { attachmentDownloadURL, attachmentsOf, isAttachmentImage } from '../src/utils/attachments.ts'

test('attachment display ignores select UIDs and previews only safe raster files', () => {
  const file: Attachment = { uid: 'fil123', name: 'photo.png', size: 100, contentType: 'image/png', url: '/_/projects/prj123/attachments/fil123' }
  assert.deepEqual(attachmentsOf([file]), [file])
  assert.deepEqual(attachmentsOf(['opt123']), [])
  assert.deepEqual(attachmentsOf(null), [])
  assert.equal(isAttachmentImage(file), true)
  assert.equal(isAttachmentImage({ ...file, contentType: 'image/svg+xml' }), false)
  assert.equal(isAttachmentImage({ ...file, contentType: 'application/pdf' }), false)
  assert.equal(attachmentDownloadURL(file), `${file.url}?download=1`)
  assert.equal(attachmentDownloadURL({ ...file, url: `${file.url}?v=1` }), `${file.url}?v=1&download=1`)
})
