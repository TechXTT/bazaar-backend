package users

import (
	"errors"
	"fmt"

	"github.com/TechXTT/bazaar-backend/modules/users/pkg/email"
	"github.com/TechXTT/bazaar-backend/modules/users/pkg/passwords"
	"github.com/TechXTT/bazaar-backend/services/db"
	"github.com/TechXTT/bazaar-backend/services/jwt"
	"github.com/gofrs/uuid/v5"
	"github.com/samber/do"
)

// NewUsersService creates a new users service
func NewUsersService(i *do.Injector) (Service, error) {
	db := do.MustInvoke[db.DB](i)
	jwks := do.MustInvoke[jwt.Jwks](i)

	return &usersService{
		db:   db,
		jwks: jwks,
	}, nil
}

func (u *usersService) CreateUser(user *Users) error {

	if err := u.save(user); err != nil {
		return err
	}

	return nil
}

func (u *usersService) UpdateUser(id string, user *Users) error {

	if err := u.update(uuid.FromStringOrNil(id), user); err != nil {
		return err
	}

	return nil
}

func (u *usersService) DeleteUser(id string) error {
	user, err := u.GetMe(id)
	if err != nil {
		return err
	}

	if err := u.delete(user); err != nil {
		return err
	}

	return nil
}

func (u *usersService) GetMe(id string) (*Users, error) {
	users := u.load()

	for _, user := range users {
		if user.ID.String() == id {
			return &user, nil
		}
	}

	return nil, errors.New("user not found")
}

func (u *usersService) LoginUser(email string, password string) (string, error) {
	users := u.load()

	for _, user := range users {
		err := passwords.ComparePassword(user.Password, password)
		if err != nil {
			return "", err
		}

		if user.Email == email {

			token, err := u.jwks.GenerateToken(user.ID.String())
			if err != nil {
				return "", err
			}

			return token, nil
		}
	}

	return "", errors.New("user not found")
}

func (u *usersService) VerifyUser(token string) error {
	id, err := u.jwks.ValidateToken(token)
	if err != nil {
		return err
	}

	users := u.load()

	for _, user := range users {
		if user.ID == uuid.FromStringOrNil(id) {
			user.EmailVerified = true
			if err := u.update(uuid.FromStringOrNil(id), &user); err != nil {
				return err
			}
			return nil
		}
	}

	return errors.New("user not found")
}

func (u *usersService) load() []Users {
	db := u.db.DB()

	var users []Users

	result := db.Find(&users)
	if result.Error != nil {
		panic(result.Error)
	}

	return users
}

func (u *usersService) save(user *Users) error {
	db := u.db.DB()

	existingUser := Users{}
	result := db.Model(&Users{}).Where("email = ?", user.Email).First(&existingUser)
	if result.RowsAffected == 1 {
		return errors.New("user already exists")
	}

	if user.Role != Admin && user.Role != Customer && user.Role != Seller {
		if user.Role == "" {
			user.Role = Customer
		} else {
			return errors.New("invalid role")
		}
	}

	hashedPassword, err := passwords.HashPassword(user.Password)
	if err != nil {
		return err
	}

	user.Password = hashedPassword

	result = db.Save(&user)
	if result.Error != nil {
		return result.Error
	}

	verificationLink, err := u.generateEmailVerificationLink(user.ID)
	if err != nil {
		db.Delete(&user)
		return err
	}

	err = email.SendEmailVerification(user.Email, user.FirstName, verificationLink)
	if err != nil {
		db.Delete(&user)
		return err
	}

	return nil
}

func (u *usersService) delete(user *Users) error {
	db := u.db.DB()

	result := db.Delete(&user)
	if result.Error != nil {
		return result.Error
	}

	return nil
}

func (u *usersService) update(id uuid.UUID, user *Users) error {
	db := u.db.DB()

	result := db.Model(&user).Omit("email", "password", "role").Where("id = ?", id).Updates(user)
	if result.Error != nil {
		return result.Error
	}

	return nil
}

func (u *usersService) generateEmailVerificationLink(id uuid.UUID) (string, error) {
	token, err := u.jwks.GenerateToken(id.String())
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("http://localhost:8000/api/users/verify-email?token=%s", token), nil
}
