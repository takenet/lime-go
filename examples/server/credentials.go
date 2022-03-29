package main

import "golang.org/x/crypto/bcrypt"

var fakeSecretStore = map[string]string{
	"john": hashPassword("mysecretpassword"),
	"mary": hashPassword("mary1234"),
}

func checkCredentials(name string, password string) bool {
	if hash, ok := fakeSecretStore[name]; ok {
		return checkPasswordHash(password, hash)
	}
	return false
}

func hashPassword(password string) string {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	if err != nil {
		panic(err)
	}
	return string(bytes)
}

func checkPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
