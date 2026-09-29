//go:generate go run github.com/swaggo/swag/cmd/swag@v1.16.6 init --parseDependency --parseInternal --parseDepth 1 --requiredByDefault -g internal/route/route.go --output docs --exclude frontend

package sayrud
