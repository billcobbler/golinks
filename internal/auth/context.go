package auth

import "context"

type contextKey string

const contextKeyUser contextKey = "auth_user"

// UserContext holds the authenticated user's identity, stored in the request context.
type UserContext struct {
	ID       int64
	Username string
}

// SetUser stores the authenticated user in the request context.
func SetUser(ctx context.Context, u UserContext) context.Context {
	return context.WithValue(ctx, contextKeyUser, u)
}

// UserFromContext retrieves the authenticated user from the context.
// Returns the zero value and false if not present (i.e. auth=none).
func UserFromContext(ctx context.Context) (UserContext, bool) {
	u, ok := ctx.Value(contextKeyUser).(UserContext)
	return u, ok
}
