import type { Profile, RequestParams, SetupAdmin, SetupDatabase, SetupDatabaseCheck, SetupPostgres, SetupStatus } from './api'
import { client } from './client'

export type { SetupAdmin, SetupDatabase, SetupDatabaseCheck, SetupStatus }
export type SetupSSLMode = SetupPostgres['sslMode']

/** Carries the one-time setup token printed in the server log. */
const withToken = (token: string): RequestParams => ({ headers: { 'X-Setup-Token': token } })

export const setupApi = {
  status: async () => (await client.setup.getSetupStatus()).data as SetupStatus,
  verify: async (token: string) => {
    await client.setup.verifySetupToken(withToken(token))
  },
  testDatabase: async (token: string, data: SetupDatabase) =>
    (await client.setup.testSetupDatabase(data, withToken(token))).data as SetupDatabaseCheck,
  saveDatabase: async (token: string, data: SetupDatabase) => {
    await client.setup.saveSetupDatabase(data, withToken(token))
  },
  createAdmin: async (token: string, data: SetupAdmin) => (await client.setup.createSetupAdmin(data, withToken(token))).data as Profile,
}
