package passwords

import "golang.org/x/crypto/bcrypt"

func HashPassword(password string) (string, error) {
	var passwordBytes = []byte(password)

	hash, err := bcrypt.GenerateFromPassword(passwordBytes, bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	return string(hash), nil
}

func ComparePassword(hashedPassword string, password string) error {
	var hashedPasswordBytes = []byte(hashedPassword)
	var passwordBytes = []byte(password)

	return bcrypt.CompareHashAndPassword(hashedPasswordBytes, passwordBytes)
}
