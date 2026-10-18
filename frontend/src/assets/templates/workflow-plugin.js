/**
 * A workflow node plugin is a single ES module. It exports:
 *   - manifest: describes the node with the same DSL as the built-in nodes;
 *   - execute(config, ctx): runs the node and returns its output;
 *   - preview(config, ctx), optional: called instead of execute by simulated debugging of action nodes.
 *
 * Texts are plain strings or translations keyed by language, e.g. { en: "Send", "zh-CN": "发送" }.
 * The module can not import other modules; there are no timers, eval or Node.js APIs.
 */
export const manifest = {
  // Unique id: lowercase letters, digits and underscores. Steps refer to the node as "plugin:<id>".
  id: "webhook_notify",
  version: "1.0.0",
  name: { en: "Webhook notification", "zh-CN": "Webhook 通知" },
  description: { en: "Posts a message to a webhook.", "zh-CN": "向 Webhook 发送一条消息。" },
  author: "Your name",
  // A Lucide icon name (https://lucide.dev/icons) or a base64 data:image URL.
  icon: "send",
  color: "#2f6bff",
  // data | logic | integration
  category: "integration",
  // Action nodes can fetch; synchronous requests wait for them, while simulated debugging only calls preview.
  action: true,
  // Time limit of a call in seconds, 1-60.
  timeout: 10,
  // Hosts that ctx.fetch can request, including their subdomains; "*" allows any public host.
  permissions: { fetch: ["hooks.example.com"] },
  // Optional credential type; projects create credentials of it, and the selected one is passed to execute with its values.
  credential: {
    label: { en: "Webhook token", "zh-CN": "Webhook 令牌" },
    summary: ["name"],
    sections: [
      {
        fields: [
          { key: "name", widget: "text", label: { en: "Name", "zh-CN": "名称" } },
          { key: "token", widget: "password", label: { en: "Token", "zh-CN": "令牌" }, required: true },
        ],
      },
    ],
  },
  // Outputs that later steps reference as steps.<stepId>.<key>.
  outputs: [
    { key: "status", label: { en: "Status code", "zh-CN": "状态码" }, kind: "number" },
    { key: "body", label: { en: "Response", "zh-CN": "响应内容" }, kind: "object" },
  ],
  // Optional branches: { output: "passed", then: "Approved", else: "Rejected" }.
  // The named output must be declared as boolean and returned by execute and preview.
  // Summary on the node card: raw:<field> shows the value as is, value:<field> describes a value field.
  summary: { parts: ["value:message"] },
  // Widgets: value (constant, variable, template or formula), number, select, credential, headers and body.
  sections: [
    {
      title: { en: "Message", "zh-CN": "消息" },
      fields: [
        { key: "credential", widget: "credential", label: { en: "Token", "zh-CN": "令牌" }, required: true },
        { key: "message", widget: "value", label: { en: "Message", "zh-CN": "消息内容" }, multiline: true, required: true },
        {
          key: "level",
          widget: "select",
          label: { en: "Level", "zh-CN": "级别" },
          default: "info",
          options: [
            { value: "info", label: { en: "Info", "zh-CN": "普通" } },
            { value: "alert", label: { en: "Alert", "zh-CN": "告警" } },
          ],
        },
      ],
    },
  ],
};

/**
 * config holds the resolved values keyed by field key; a credential field holds the credential values.
 * ctx provides:
 *   - ctx.fetch(url, { method, headers, body }): sends a request to an allowed host, a body object is sent as JSON;
 *     the response has status, ok, headers, text() and json();
 *   - ctx.log(...values): writes to the step logs, like console.log;
 *   - ctx.crypto.sha256(text): returns the SHA-256 hex digest of UTF-8 text;
 *   - ctx.crypto.hmacSha256(key, text, keyEncoding = "utf8"): returns a hex digest; use "hex" for a derived key;
 *   - ctx.run, ctx.step, ctx.project and ctx.plugin: information about the execution;
 *   - ctx.preview: true when called for simulated debugging.
 * Throw an error to fail the step; set error.retryable = true to let background flows retry it.
 */
export async function execute(config, ctx) {
  const resp = await ctx.fetch("https://hooks.example.com/notify", {
    method: "POST",
    headers: { Authorization: "Bearer " + config.credential.token },
    body: { text: config.message, level: config.level },
  });
  ctx.log("webhook responded", resp.status);
  if (!resp.ok) {
    const error = new Error("the webhook responded " + resp.status);
    error.retryable = resp.status >= 500;
    throw error;
  }
  let body;
  try {
    body = resp.json();
  } catch {
    body = resp.text();
  }
  return { status: resp.status, body };
}

export function preview(config) {
  return { message: config.message, level: config.level };
}
