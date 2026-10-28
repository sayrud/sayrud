import type { CredentialTypeDef, ModuleDef } from '@/types/workflow'
import type { WorkflowPlugin as ApiPlugin, TestWorkflowPluginResp } from './api'
import { client } from './client'

export type { TestWorkflowPluginResp }

export type PluginLanguage = 'javascript' | 'typescript'

/** An installed node plugin; the generated types describe the DSL loosely, so it is narrowed to the editor types. */
export type WorkflowPlugin = Omit<ApiPlugin, 'module' | 'credential' | 'language'> & {
  module: ModuleDef
  credential?: CredentialTypeDef
  language: PluginLanguage
}

export interface WorkflowPluginList {
  plugins: WorkflowPlugin[]
  /** Built-in nodes, whose texts are i18n keys. */
  builtins: ModuleDef[]
}

export interface PluginInspection {
  plugin: WorkflowPlugin
  /** The installed plugin with the same id that the source would replace. */
  installed?: WorkflowPlugin
  source: string
  language: PluginLanguage
  hasPreview: boolean
  /** Verified Sayrud Hub signature of the source to send back when installing, absent if the source is not from the Hub. */
  signature?: object
}

export interface PluginSource {
  source?: string
  language?: PluginLanguage
  url?: string
  /** Downloads a release of the plugin of this id from Sayrud Hub. */
  hubID?: string
  /** Exact release to inspect, latest when omitted. */
  hubVersion?: string
}

export interface TestPluginRequest {
  /** Runs the installed plugin when no source is given. */
  pluginID?: string
  source?: string
  language?: PluginLanguage
  config: Record<string, unknown>
}

export const workflowPluginsApi = {
  list: async () => (await client.admin.listAdminWorkflowPlugins()).data as unknown as WorkflowPluginList,
  get: async (id: string) => (await client.admin.getAdminWorkflowPlugin(id)).data as unknown as WorkflowPlugin,
  inspect: async (body: PluginSource) => (await client.admin.inspectAdminWorkflowPlugin(body)).data as unknown as PluginInspection,
  install: async (source: string, language: PluginLanguage, sourceURL?: string, signature?: object) =>
    (await client.admin.installAdminWorkflowPlugin({ source, language, sourceURL, signature })).data as unknown as WorkflowPlugin,
  setEnabled: async (id: string, enabled: boolean) =>
    (await client.admin.setAdminWorkflowPluginEnabled(id, { enabled })).data as unknown as WorkflowPlugin,
  uninstall: async (id: string) => {
    await client.admin.uninstallAdminWorkflowPlugin(id)
  },
  test: async (body: TestPluginRequest) => (await client.admin.testAdminWorkflowPlugin(body)).data,
}
