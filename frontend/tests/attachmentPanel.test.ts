import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import test from 'node:test'
import ts from 'typescript'
import { createRenderer, defineComponent, h, nextTick } from 'vue'
import { compileScript, parse } from 'vue/compiler-sfc'

import type { Attachment, CellValue, SLField } from '../src/types/bitable.ts'
import * as attachments from '../src/utils/attachments.ts'

test('attachment uploads, previews and reordering preserve readonly and busy guards', async () => {
  const source = readFileSync(new URL('../src/components/cell/AttachmentPanel.vue', import.meta.url), 'utf8')
  const script = compileScript(parse(source).descriptor, { id: 'attachment-panel-test', inlineTemplate: true })
  const compiled = ts.transpileModule(script.content, { compilerOptions: { module: ts.ModuleKind.CommonJS } }).outputText
  const module = { exports: {} as { default?: ReturnType<typeof defineComponent> } }
  const require = createRequire(import.meta.url)
  const uploads: File[] = []
  const busy: boolean[] = []
  const changes: CellValue[] = []
  const previews: boolean[] = []
  const warnings: string[] = []
  let finishUpload: (file: Attachment) => void = () => {}
  let beforeUpload: (file: File) => boolean = () => false
  const icon = defineComponent(() => () => h('icon'))
  const slotStub = defineComponent((_props, { slots }) => () => h('stub', [slots.icon?.(), slots.image?.(), slots.default?.()]))
  const uploadStub = defineComponent({
    props: ['onBeforeUpload'],
    setup(props, { slots }) {
      return () => {
        beforeUpload = props.onBeforeUpload
        return h('upload', slots['upload-button']?.())
      }
    },
  })
  const overrides: Record<string, unknown> = {
    '@arco-design/web-vue': { Message: { warning: (message: string) => warnings.push(message), error: assert.fail } },
    '@lucide/vue': { Download: icon, Eye: icon, File: icon, Move: icon, Plus: icon, X: icon },
    'vue-i18n': { useI18n: () => ({ t: (key: string) => key }) },
    '@/api/bitable': {
      attachmentsApi: {
        upload: (_project: string, _field: SLField, file: File) => {
          uploads.push(file)
          return new Promise<Attachment>((resolve) => { finishUpload = resolve })
        },
      },
    },
    '@/stores/base': { useBaseStore: () => ({ canEdit: true, project: { uid: 'prj123' } }) },
    '@/utils/attachments': attachments,
  }
  new Function('require', 'module', 'exports', compiled)((id: string) => overrides[id] ?? require(id), module, module.exports)

  function transfer(files: File[]) {
    let prevented = false
    let stopped = false
    return {
      clipboardData: { files },
      dataTransfer: { files, setData: () => {}, effectAllowed: '', dropEffect: '' },
      currentTarget: { closest: () => null },
      preventDefault: () => { prevented = true },
      stopPropagation: () => { stopped = true },
      get prevented() { return prevented },
      get stopped() { return stopped },
    }
  }
  type Handler = (event: ReturnType<typeof transfer>) => void | Promise<void>
  const handlers: Record<string, Handler> = {}
  type Element = { attrs: Record<string, unknown> }
  const elements: Element[] = []
  function fire(element: Element, key: string, event = transfer([])) {
    const callbacks = element.attrs[key]
    for (const callback of Array.isArray(callbacks) ? callbacks : [callbacks]) (callback as Handler)(event)
    return event
  }
  const handles = () => elements.filter((element) => String(element.attrs.class).includes('reorder-handle'))
  const card = (uid: string) => elements.find((element) => element.attrs['data-attachment'] === uid)!
  let panelElement: unknown
  const renderer = createRenderer({
    createElement: () => {
      const element: Element = { attrs: {} }
      elements.push(element)
      return element
    },
    createText: () => ({ attrs: {} }), createComment: () => ({ attrs: {} }),
    insert: () => {}, remove: () => {}, setText: () => {}, setElementText: () => {},
    patchProp: (element, key, _previous, value) => {
      element.attrs[key] = value
      if (key === 'class' && value === 'attachment-panel') panelElement = element
      if (element === panelElement && (key === 'onPaste' || key === 'onDrop')) handlers[key] = value as Handler
    },
    parentNode: () => ({ attrs: {} }), nextSibling: () => null,
  })
  const props = {
    field: { uid: 'fld123', tableUID: 'tbl123', type: 'attachment', label: 'Attachments', metadata: {} } as SLField,
    value: null as CellValue,
  }
  function mount(readonly = false, value: CellValue = null) {
    elements.length = 0
    const app = renderer.createApp({
      render: () => h(module.exports.default!, {
        ...props,
        value,
        readonly,
        onBusy: (value: boolean) => busy.push(value),
        onChange: (value: CellValue) => changes.push(value),
        onPreview: (value: boolean) => previews.push(value),
      }),
    })
    app.component('a-image', icon)
    app.component('a-upload', uploadStub)
    for (const name of ['a-card', 'a-button', 'a-empty', 'a-image-preview']) app.component(name, slotStub)
    app.mount({ attrs: {} })
    return app
  }
  let app = mount()

  try {
    const text = transfer([])
    await handlers.onPaste!(text)
    assert.equal(text.prevented, false)
    assert.equal(text.stopped, false)
    assert.equal(uploads.length, 0)

    const file = new File(['photo'], 'photo.png', { type: 'image/png' })
    const pasted = transfer([file])
    const pasting = handlers.onPaste!(pasted)
    assert.equal(pasted.prevented, true)
    assert.equal(pasted.stopped, true)
    assert.deepEqual(uploads, [file])
    assert.deepEqual(busy, [true])

    const whileBusy = transfer([file])
    await handlers.onDrop!(whileBusy)
    assert.equal(whileBusy.prevented, true)
    assert.equal(uploads.length, 1)

    const attachment: Attachment = { uid: 'fil123', name: file.name, size: file.size, contentType: file.type, url: '/_/projects/prj123/attachments/fil123' }
    finishUpload(attachment)
    await pasting
    assert.deepEqual(changes, [[attachment]])
    assert.deepEqual(busy, [true, false])

    const dropped = transfer([file])
    const dropping = handlers.onDrop!(dropped)
    assert.equal(dropped.prevented, true)
    assert.equal(dropped.stopped, true)
    assert.deepEqual(uploads, [file, file])
    finishUpload(attachment)
    await dropping
    assert.deepEqual(changes, [[attachment], [attachment]])
    assert.deepEqual(busy, [true, false, true, false])

    const document = new File(['notes'], 'notes.txt', { type: 'text/plain' })
    assert.equal(beforeUpload(file), false)
    assert.equal(beforeUpload(document), false)
    await Promise.resolve()
    assert.deepEqual(uploads, [file, file, file])
    assert.deepEqual(busy, [true, false, true, false, true])
    finishUpload(attachment)
    await Promise.resolve()
    assert.deepEqual(uploads, [file, file, file, document])
    const second: Attachment = { ...attachment, uid: 'fil456', name: document.name, contentType: document.type }
    finishUpload(second)
    await Promise.resolve()
    assert.deepEqual(changes.at(-1), [attachment, second])
    assert.deepEqual(busy, [true, false, true, false, true, false])

    for (let i = 0; i <= attachments.ATTACHMENT_MAX_COUNT; i++) beforeUpload(file)
    await Promise.resolve()
    assert.equal(uploads.length, 4)
    assert.deepEqual(warnings, ['cell.attachmentLimitReached'])

    app.unmount()
    app = mount(true)
    const readonlyDrop = transfer([file])
    await handlers.onDrop!(readonlyDrop)
    assert.equal(readonlyDrop.prevented, true)
    assert.equal(uploads.length, 4)

    app.unmount()
    const original = [attachment, second]
    app = mount(false, original)
    changes.length = 0
    const eye = elements.find((element) => element.attrs.title === 'cell.previewAttachment')!
    fire(eye, 'onClick')
    assert.deepEqual(previews, [true])
    await nextTick()
    fire(elements.find((element) => element.attrs.onClose)!, 'onClose')
    assert.deepEqual(previews, [true, false])
    await nextTick()

    fire(handles()[0]!, 'onDragstart')
    const moving = fire(card(second.uid), 'onDrop', transfer([file]))
    assert.equal(moving.prevented, true)
    assert.equal(moving.stopped, true)
    const reordered = changes.at(-1) as Attachment[]
    assert.deepEqual(reordered, [second, attachment])
    assert.notEqual(reordered, original)
    assert.deepEqual(original, [attachment, second])
    assert.equal(new Set(reordered.map((item) => item.uid)).size, 2)
    assert.equal(uploads.length, 4)

    app.unmount()
    app = mount(false, reordered)
    fire(handles()[0]!, 'onDragstart')
    fire(card(attachment.uid), 'onDrop')
    assert.deepEqual(changes.at(-1), original)
    const changedCount = changes.length
    fire(handles()[0]!, 'onDragstart')
    fire(card(second.uid), 'onDrop')
    assert.equal(changes.length, changedCount)

    fire(handles()[0]!, 'onDragstart')
    const uploading = handlers.onPaste!(transfer([file]))
    const blockedStart = fire(handles()[1]!, 'onDragstart')
    assert.equal(blockedStart.prevented, true)
    fire(card(attachment.uid), 'onDrop')
    assert.equal(changes.length, changedCount)
    assert.equal(uploads.length, 5)
    finishUpload({ ...attachment, uid: 'fil789' })
    await uploading

    app.unmount()
    app = mount(true, original)
    const readonlyChanges = changes.length
    assert.equal(handles().length, 0)
    fire(card(second.uid), 'onDrop')
    assert.equal(changes.length, readonlyChanges)
    assert.equal(uploads.length, 5)
  } finally {
    app.unmount()
  }
})
