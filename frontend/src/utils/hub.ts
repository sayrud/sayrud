import type { HubPackage, HubReleaseDetail } from '@/api/hub'

/**
 * What can be done with a package of the catalog:
 * install it, update or open its installation from the Hub, or nothing because no release is available, the release
 * requires a later Sayrud, or a workflow plugin with the same id is installed from elsewhere and must not be replaced.
 */
export type HubAction = 'install' | 'update' | 'open' | 'conflict' | 'unavailable' | 'incompatible'

/** Tells the action of the package, latest is the release detail when known, which tells its compatibility. */
export function hubAction(p: HubPackage, latest?: HubReleaseDetail): HubAction {
  if (!p.latestVersion) return 'unavailable'
  const installed = p.installed
  // A field shortcut can be installed several times, a plugin id identifies a single node.
  if (installed && !installed.fromHub && p.kind === 'workflow_plugin') return 'conflict'
  const current = !!installed?.fromHub && installed.version === p.latestVersion
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
