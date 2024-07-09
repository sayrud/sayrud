export const MiddlewareSelectList: Record<string, string> = {
    'rate_limit': '频次限流',
    'send_email': '发送邮件',
}
export const MiddlewareNames: Record<string, string> = {
    'main': '数据操作',
    ...MiddlewareSelectList,
}

export const MiddlewareIcons: Record<string, string> = {
    'main': 'api',
    'rate_limit': 'map-ruler',
    'send_email': 'mail',
}
