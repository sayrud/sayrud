import { t } from '../i18n/translate.ts'

export type DeviceKind = 'desktop' | 'mobile' | 'tablet'

export interface ParsedUserAgent {
  browser: string
  os: string
  device: DeviceKind
}

// The order matters: the UA of Edge and Opera contains Chrome, Chrome contains Safari, and Android contains Linux.
const BROWSERS: [RegExp, string][] = [
  [/Edg(e|A|iOS)?\//, 'Edge'],
  [/OPR\/|Opera/, 'Opera'],
  [/Firefox\/|FxiOS\//, 'Firefox'],
  [/Chrome\/|CriOS\//, 'Chrome'],
  [/Safari\//, 'Safari'],
]

const SYSTEMS: [RegExp, string][] = [
  [/iPhone|iPad|iPod/, 'iOS'],
  [/Android/, 'Android'],
  [/Mac OS X|Macintosh/, 'macOS'],
  [/Windows/, 'Windows'],
  [/CrOS/, 'ChromeOS'],
  [/Linux/, 'Linux'],
]

/** Roughly detects the browser, OS and device type from the user agent, for the signed-in device list. */
export function parseUserAgent(ua: string): ParsedUserAgent {
  const browser = BROWSERS.find(([re]) => re.test(ua))?.[1] ?? t('userAgent.unknownBrowser')
  const os = SYSTEMS.find(([re]) => re.test(ua))?.[1] ?? t('userAgent.unknownOS')
  let device: DeviceKind = 'desktop'
  if (/iPad|Tablet/.test(ua) || (/Android/.test(ua) && !/Mobile/.test(ua))) device = 'tablet'
  else if (/Mobile|iPhone|iPod|Android/.test(ua)) device = 'mobile'
  return { browser, os, device }
}
