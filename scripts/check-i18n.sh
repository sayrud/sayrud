#!/usr/bin/env bash
set -uo pipefail

cd "$(dirname "$0")/.." || exit 1

# Run both sides even when one fails, so a CI run shows the complete report.
status=0
pnpm --dir frontend i18n:check || status=1
pnpm --dir frontend exec node --experimental-transform-types --test tests/locales.test.ts tests/i18nKeys.test.ts tests/i18nAudit.test.ts || status=1
go test ./internal/i18n -count=1 || status=1
exit "$status"
