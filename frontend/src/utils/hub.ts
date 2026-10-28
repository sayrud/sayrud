import type { HubPackage, HubReleaseDetail } from '@/api/hub'

/**
 * What can be done with a package of the catalog:
 * install it, update or open its installation from the Hub, or nothing because no release is available, the release
 * requires a later Sayrud, or a workflow plugin with the same id is installed from elsewhere and must not be replaced.
 */
export type HubAction = 'install' | 'update' | 'open' | 'conflict' | 'unavailable' | 'incompatible'

/** Compares release versions without counting build metadata as an update. */
function isNewerRelease(next: string, current: string): boolean {
  const version = /^(0|[1-9]\d*)(?:\.(0|[1-9]\d*))?(?:\.(0|[1-9]\d*))?(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$/
  const a = version.exec(next)
  const b = version.exec(current)
  if (!a || !b) return false
  for (let i = 1; i <= 3; i++) {
    const left = BigInt(a[i] ?? '0')
    const right = BigInt(b[i] ?? '0')
    if (left !== right) return left > right
  }
  const prereleaseA = a[4]?.split('.') ?? []
  const prereleaseB = b[4]?.split('.') ?? []
  if (!prereleaseA.length || !prereleaseB.length) return !prereleaseA.length && !!prereleaseB.length
  for (let i = 0; i < Math.min(prereleaseA.length, prereleaseB.length); i++) {
    const left = prereleaseA[i]!
    const right = prereleaseB[i]!
    if (left === right) continue
    const numericA = /^\d+$/.test(left)
    const numericB = /^\d+$/.test(right)
    if (numericA && numericB) return BigInt(left) > BigInt(right)
    if (numericA !== numericB) return !numericA
    return left > right
  }
  return prereleaseA.length > prereleaseB.length
}

/** Tells the action of the package, latest is the release detail when known, which tells its compatibility. */
export function hubAction(p: HubPackage, latest?: HubReleaseDetail): HubAction {
  if (!p.latestVersion) return 'unavailable'
  const installed = p.installed
  // A field shortcut can be installed several times, a plugin id identifies a single node.
  if (installed && !installed.fromHub && p.kind === 'workflow_plugin') return 'conflict'
  const current = !!installed?.fromHub && !isNewerRelease(p.latestVersion, installed.version)
  if (current) return 'open'
  if (latest && !latest.compatible) return 'incompatible'
  return installed?.fromHub ? 'update' : 'install'
}

/** Formats a count such as the downloads briefly, like 1.2K. */
export function compactCount(n: number, locale: string): string {
  try {
    return new Intl.NumberFormat(locale, { notation: 'compact', maximumFractionDigits: 1 }).format(n)
  } catch {
    return String(n)
  }
}
