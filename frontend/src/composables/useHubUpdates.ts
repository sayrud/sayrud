import { computed, ref } from 'vue'

import { hubApi, type HubKind, type HubOrigin, type HubUpdate } from '@/api/hub'

// The state is shared by the pages so that Sayrud Hub is checked once per session.
const updates = ref<HubUpdate[]>([])
const checkedAt = ref('')
const checking = ref(false)
const error = ref('')
let pending: Promise<boolean> | null = null
let loaded = false

/** Starts a new check; refresh also makes the backend ask the Hub again. Resolves to whether it succeeded. */
function check(refresh = false): Promise<boolean> {
  loaded = true
  checking.value = true
  const request = (pending = hubApi
    .updates(refresh)
    .then((resp) => {
      if (pending !== request) return true
      updates.value = resp.packages
      checkedAt.value = resp.checkedAt
      error.value = ''
      return true
    })
    .catch((e: unknown) => {
      if (pending === request) error.value = e instanceof Error ? e.message : String(e)
      return false
    })
    .finally(() => {
      if (pending === request) {
        checking.value = false
        pending = null
      }
    }))
  return request
}

/** Checks the updates unless they have been checked in this session. */
function ensure() {
  if (!loaded) void check()
}

/** Returns the state on the Hub of the installed package, undefined if it is not installed from the Hub or has nothing to report. */
function find(kind: HubKind, id: string, hub: HubOrigin | undefined): HubUpdate | undefined {
  if (!hub) return undefined
  return updates.value.find((u) => u.kind === kind && u.id === id && u.version === hub.version)
}

/** Shared state of the packages installed from Sayrud Hub. */
export function useHubUpdates(kind?: HubKind) {
  const ofKind = computed(() => (kind ? updates.value.filter((u) => u.kind === kind) : updates.value))
  return {
    updates: ofKind,
    /** Installed versions which are yanked or whose package is delisted. */
    notices: computed(() => ofKind.value.filter((u) => u.yanked || u.delisted)),
    /** Number of the packages with a newer release. */
    available: computed(() => ofKind.value.filter((u) => u.latest).length),
    checkedAt,
    checking,
    error,
    check,
    ensure,
    find,
  }
}

/** Forgets the state, only for the tests. */
export function resetHubUpdates() {
  updates.value = []
  checkedAt.value = ''
  checking.value = false
  error.value = ''
  pending = null
  loaded = false
}
