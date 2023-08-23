package token

const minSecretKeySize = 32

// JWTMaker is a JSON Web Token maker
type JWTMaker struct {
	secretKey string
}

// NewJWTMaker creates a new JWTMaker
func NewJWTMaker(secretKey string) (Maker, error) {
	if len(secretKey) < minSecretKeySize {
		return nil, fnt.Errorf("invalid key size: must be at least %d characters", minSecretKeySize)

	}

	return &JWTMaker(secretKey), nil
}
