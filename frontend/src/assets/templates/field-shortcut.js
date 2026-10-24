/**
 * A field shortcut is a single JavaScript or TypeScript module exporting manifest and execute.
 * Imports and browser or Node.js APIs are unavailable.
 * Install from a .js/.mjs/.ts/.mts file or a URL pointing directly to this source.
 */
export const manifest = {
  name: "Uppercase text",
  version: "1.0.0",
  description: "Converts the selected field's text to uppercase.",
  // text | number | checkbox | datetime | single_select | multi_select
  resultType: "text",
  formItems: [
    {
      key: "text",
      // Display text accepts a string or translations, just like workflow nodes.
      label: { en: "Text field", "zh-CN": "文本字段" },
      component: "field_select",
      fieldTypes: ["text"],
      required: true,
    },
  ],
  // Hosts context.fetch can request, including their subdomains.
  domains: [],
  // Definitions only, e.g. { key: "api", label: { en: "API Key", "zh-CN": "API 密钥" }, type: "bearer" }.
  // Configure secret values in the shortcut editor after installation.
  credentials: [],
  // Single execution limit: 1–300 seconds.
  timeoutSeconds: 30,
  // Allows context.ai.complete to use the globally configured model.
  aiEnabled: false,
};

/**
 * params contains the configured form values; field references resolve to cell values.
 * context.fetch(url, options, credentialKey) requests an allowed host.
 * context.ai.complete({ prompt, system }) calls the global AI model when aiEnabled.
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
