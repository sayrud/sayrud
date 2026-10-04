export const AI_PROVIDERS = [
  { id: 'openai', name: 'OpenAI', icon: 'openai', baseURL: 'https://api.openai.com/v1' },
  { id: 'deepseek', name: 'DeepSeek', icon: 'deepseek', baseURL: 'https://api.deepseek.com/v1' },
  { id: 'tencent-tokenhub', name: 'Tencent Cloud TokenHub', icon: 'tokenhub', baseURL: 'https://tokenhub.tencentmaas.com/v1' },
  { id: 'moonshot-cn', name: 'Kimi', icon: 'kimi', baseURL: 'https://api.moonshot.cn/v1' },
  { id: 'zhipu', name: 'Zhipu GLM', icon: 'zhipu', baseURL: 'https://open.bigmodel.cn/api/paas/v4' },
  { id: 'qwen-cn', name: 'Qwen', icon: 'qwen', baseURL: 'https://dashscope.aliyuncs.com/compatible-mode/v1' },
  { id: 'minimax-cn', name: 'MiniMax', icon: 'minimax', baseURL: 'https://api.minimaxi.com/v1' },
  { id: 'siliconflow', name: 'SiliconFlow', icon: 'siliconflow', baseURL: 'https://api.siliconflow.cn/v1' },
  { id: 'openrouter', name: 'OpenRouter', icon: 'openrouter', baseURL: 'https://openrouter.ai/api/v1' },
  { id: 'google', name: 'Google Gemini', icon: 'gemini', baseURL: 'https://generativelanguage.googleapis.com/v1beta/openai' },
  { id: 'groq', name: 'Groq', icon: 'groq', baseURL: 'https://api.groq.com/openai/v1' },
  { id: 'ollama', name: 'Ollama', icon: 'ollama', baseURL: 'http://localhost:11434/v1' },
  { id: 'lmstudio', name: 'LM Studio', icon: 'lmstudio', baseURL: 'http://localhost:1234/v1' },
]

export function normalizeAIBaseURL(baseURL: string) {
  return baseURL.trim().replace(/\/+$/, '')
}

export function findAIProvider(baseURL: string) {
  return AI_PROVIDERS.find((provider) => provider.baseURL === normalizeAIBaseURL(baseURL))
}
