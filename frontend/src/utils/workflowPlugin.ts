import type { CredentialTypeDef, FieldDef, ModuleDef } from '../types/workflow.ts'

export type PluginLanguage = 'javascript' | 'typescript'

/** Largest plugin source accepted by the backend, in bytes. */
export const MAX_PLUGIN_SOURCE = 512 * 1024

/** Display names of the source languages, which keep their original spelling. */
export const LANGUAGE_NAMES: Record<PluginLanguage, string> = { javascript: 'JavaScript', typescript: 'TypeScript' }

/** Language of a plugin file by its extension. */
export function languageOf(name: string): PluginLanguage {
  return /\.m?ts$/i.test(name) ? 'typescript' : 'javascript'
}

/** Whether a module icon is an embedded image rather than a Lucide icon name. */
export function isImageIcon(icon: string | undefined): icon is string {
  return !!icon && /^data:image\/(svg\+xml|png|jpeg|webp);base64,/.test(icon)
}

function sampleValue(f: FieldDef, credential?: CredentialTypeDef): unknown {
  if (f.default !== undefined && f.widget !== 'value') return f.default
  switch (f.widget) {
    case 'number':
      return f.min ?? 0
    case 'select':
      return f.options?.[0]?.value ?? ''
    case 'headers':
    case 'body':
      return {}
    case 'credential':
      if (credential && f.credentialType === credential.type) {
        return Object.fromEntries(credential.sections.flatMap((s) => s.fields).filter((c) => c.key).map((c) => [c.key!, '']))
      }
      return {}
  }
  return ''
}

/** Config to test a plugin with: values are given as is, so value fields hold their result and credential fields hold the credential values. */
export function sampleConfig(def: ModuleDef, credential?: CredentialTypeDef): Record<string, unknown> {
  const out: Record<string, unknown> = {}
  for (const f of def.sections.flatMap((s) => s.fields)) {
    if (f.key) out[f.key] = sampleValue(f, credential)
  }
  return out
}

/** Version as shown next to a plugin name, e.g. v1.2.0. */
export function versionLabel(version: string) {
  return `v${version}`
}

/** Source file name of a plugin, used when downloading it. */
export function pluginFileName(id: string, language: PluginLanguage) {
  return `${id}${language === 'typescript' ? '.ts' : '.js'}`
}
