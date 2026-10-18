package sandbox

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCrypto(t *testing.T) {
	for _, tc := range []struct {
		expr string
		want string
	}{
		{`ctx.crypto.sha256("abc")`, "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"},
		{`ctx.crypto.hmacSha256("0b".repeat(20), "Hi There", "hex")`, "b0344c61d8db38535ca8afceaf0bf12b881dc200c9833da726e9376c2e32cff7"},
		{`ctx.crypto.hmacSha256("Jefe", "what do ya want for nothing?")`, "5bdcc146bf60754e6a042426089575c75a003f089d2739839dec58b964ec3843"},
	} {
		p := compile(t, `export const manifest = {}; export function execute(_, ctx) { return `+tc.expr+`; }`)
		result, err := p.Run(context.Background(), Call{Hook: HookExecute, Args: []interface{}{nil}})
		require.NoError(t, err)
		assert.Equal(t, tc.want, result.Value)
	}

	for _, expr := range []string{
		`ctx.crypto.sha256(1)`,
		`ctx.crypto.hmacSha256(null, "text")`,
		`ctx.crypto.hmacSha256("key", 1)`,
		`ctx.crypto.hmacSha256("key", "text", "base64")`,
		`ctx.crypto.hmacSha256("private-invalid-key", "text", "hex")`,
	} {
		p := compile(t, `export const manifest = {}; export function execute(_, ctx) { return `+expr+`; }`)
		_, err := p.Run(context.Background(), Call{Hook: HookExecute, Args: []interface{}{nil}})
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "private-invalid-key")
	}
}
