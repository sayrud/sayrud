import { Modal } from '@arco-design/web-vue'
import { ref, type Ref } from 'vue'

import { t } from '@/i18n'

/** Asks before closing a one-time secret that has not been copied, close runs once confirmed. */
export function useSecretGuard(secret: Ref<string | null | undefined>, close: () => void) {
  const copied = ref(false)

  function requestClose() {
    if (!secret.value || copied.value) {
      close()
      return
    }
    Modal.warning({
      title: t('accessToken.closeConfirmTitle'),
      content: t('accessToken.closeConfirmContent'),
      hideCancel: false,
      okText: t('common.close'),
      onOk: close,
    })
  }

  return { copied, requestClose }
}
