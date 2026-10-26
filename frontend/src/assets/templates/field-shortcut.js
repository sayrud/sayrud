/**
 * A field shortcut is a single JavaScript or TypeScript module. It exports:
 *   - manifest: describes the shortcut, with the properties shared with workflow node plugins named the same way;
 *   - execute(params, context): returns the cell value.
 *
 * Texts are plain strings or translations keyed by language codes such as en and zh-CN.
 * Imports and browser or Node.js APIs are unavailable.
 * Install from a .js/.mjs/.ts/.mts file or a URL pointing directly to this source.
 */
export const manifest = {
  // Source package id: lowercase letters, digits and underscores. Installed shortcuts have separate local UIDs.
  id: "uppercase_text",
  version: "1.0.0",
  name: { en: "Uppercase text", "zh-CN": "转为大写" },
  description: { en: "Converts the selected field's text to uppercase.", "zh-CN": "将所选字段的文本转为大写。" },
  // Optional project website or support page.
  homepage: "",
  // text | number | checkbox | datetime | single_select | multi_select
  resultType: "text",
  // Time limit of an execution in seconds, 1-300.
  timeout: 30,
  // fetch: hosts that context.fetch can request, including their subdomains; "*" allows any public host.
  // ai: allows context.ai.complete to use the globally configured model.
  permissions: { fetch: [], ai: false },
  // Definitions only, e.g. { key: "api", label: { en: "API Key", "zh-CN": "API 密钥" }, type: "bearer" }.
  // type is bearer, header or query; header and query also need the name of the header or query parameter.
  // Configure secret values in the shortcut editor after installation.
  credentials: [],
  // Widgets: field_select (a field of the record), text (multiline for a text area), select,
  // prompt (a text referencing fields) and field_options (the options of a select field being configured).
  sections: [
    {
      fields: [
        {
          key: "text",
          widget: "field_select",
          label: { en: "Text field", "zh-CN": "文本字段" },
          fieldTypes: ["text"],
          required: true,
        },
      ],
    },
  ],
};

/**
 * params contains the configured form values keyed by field key; field references resolve to cell values.
 * context.fetch(url, options, credentialKey) requests an allowed host.
 * context.ai.complete({ prompt, system }) calls the global AI model when permissions.ai is true.
 * context.log(...values) writes execution logs without exposing stored secrets.
 * Return a value matching resultType, or throw an error to fail the cell.
 * For translated errors, attach your module's translations and parameters:
 * throw Object.assign(new Error("Request failed"), {
 *   i18n: {
 *     key: "request_failed",
 *     messages: { en: "Request failed (HTTP {status}).", "zh-CN": "请求失败（HTTP {status}）。" },
 *     params: { status: 429 },
 *   },
 * });
 * The viewer's language is selected when displaying the error, including stored jobs.
 * Missing translations fall back to the same base language, English, then another translation.
 */
export async function execute(params) {
  return String(params.text ?? "").toUpperCase();
}
