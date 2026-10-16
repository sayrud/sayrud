import { Modal } from '@arco-design/web-vue'
import { h } from 'vue'
import { useI18n } from 'vue-i18n'

import { useWorkflowEditorStore } from '@/stores/workflowEditor'
import type { TriggerType } from '@/types/workflow'
import { switchTrigger, TRIGGER_META, triggerSwitchLoss } from '@/utils/workflow'

/** Change the trigger type, asking for confirmation first when API-specific configuration would be removed. */
export function useTriggerSwitch() {
  const { t } = useI18n()
  const store = useWorkflowEditorStore()

  function apply(to: TriggerType) {
    const spec = store.spec!
    switchTrigger(spec, to, store.fields, store.tables[0]?.uid)
    void store.ensureFields(spec.trigger.record?.tableUID)
    store.select({ kind: 'trigger' })
  }

  function change(to: TriggerType) {
    const spec = store.spec
    if (!spec || to === spec.trigger.type) return
    const loss = triggerSwitchLoss(spec, to)
    const items = [
      loss.params && t('workflow.trigger.lossParams', { n: loss.params }),
      loss.responses && t('workflow.trigger.lossResponses', { n: loss.responses }),
      loss.async && t('workflow.trigger.lossAsync', { n: loss.async }),
    ].filter((x): x is string => !!x)
    if (!items.length) return apply(to)
    Modal.warning({
      title: t('workflow.trigger.switchTitle', { name: TRIGGER_META[to].label }),
      content: () => h('div', [h('div', t('workflow.trigger.switchContent')), h('ul', { class: 'switch-loss' }, items.map((x) => h('li', x)))]),
      hideCancel: false,
      okText: t('workflow.trigger.switchOk'),
      onOk: () => apply(to),
    })
  }

  return { change }
}
