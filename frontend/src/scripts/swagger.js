import * as path from 'node:path'
import * as process from 'node:process'
import { generateApi } from 'swagger-typescript-api'

await generateApi({
  input: path.resolve(process.cwd(), '../docs/swagger.json'),
  output: path.resolve(process.cwd(), './src/api'),
  fileName: 'api.ts',
  generateClient: true,
  httpClientType: 'axios',
  extractRequestBody: true,
  extractResponseBody: true,
})
