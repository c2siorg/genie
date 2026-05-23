package auth

import "golang.org/x/crypto/bcrypt"

// DefaultCost is bcrypt's default work factor (10). Bump in production to
// raise the per-attempt cost.
const DefaultCost = bcrypt.DefaultCost

// HashPassword returns a bcrypt hash suitable for storing in User.PasswordHash.
func HashPassword(plain string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(plain), DefaultCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

// VerifyPassword returns nil if the plain password matches the stored hash.
func VerifyPassword(hash, plain string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
}
