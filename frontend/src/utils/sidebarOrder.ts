/** Sort the shared positions; an optimistic move temporarily overrides them until saved. */
export function orderSidebarItems<T extends { uid: string; position: number }>(items: T[], order: readonly string[] = []): T[] {
  const remaining = new Map([...items].sort((a, b) => a.position - b.position).map((item) => [item.uid, item]))
  const result: T[] = []

  for (const uid of order) {
    const item = remaining.get(uid)
    if (!item) continue

    result.push(item)
    remaining.delete(uid)
  }

  return [...result, ...remaining.values()]
}

/** Move against the full list so filtering never discards hidden entries. */
export function moveSidebarItem(order: readonly string[], uid: string, target: string, after: boolean): string[] {
  if (uid === target || !order.includes(uid) || !order.includes(target)) return [...order]

  const next = order.filter((item) => item !== uid)
  next.splice(next.indexOf(target) + Number(after), 0, uid)

  return next
}
