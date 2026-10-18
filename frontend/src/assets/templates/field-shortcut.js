/**
 * A field shortcut is a single JavaScript or TypeScript module exporting manifest and execute.
 * Imports and browser or Node.js APIs are unavailable.
 * Install from a .js/.mjs/.ts/.mts file or a URL pointing directly to this source.
 */
export const manifest = {
  name: "Uppercase text",
  description: "Converts the selected field's text to uppercase.",
  // text | number | checkbox | datetime | single_select | multi_select
  resultType: "text",
  formItems: [
    {
      key: "text",
      label: "Text field",
      component: "field_select",
      fieldTypes: ["text"],
      required: true,
    },
  ],
  // Hosts context.fetch can request, including their subdomains.
  domains: [],
  // Definitions only, e.g. { key: "api", type: "bearer" }.
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
 */
export async function execute(params) {
  return String(params.text ?? "").toUpperCase();
}
