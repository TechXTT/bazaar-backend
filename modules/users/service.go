package users

import (
	"errors"
	"log"

	"github.com/TechXTT/bazaar-backend/services/db"
	"github.com/TechXTT/bazaar-backend/services/jwt"
	"github.com/gofrs/uuid/v5"
	"github.com/samber/do"
)

// NewUsersService creates a new users service
func NewUsersService(i *do.Injector) (Service, error) {
	db := do.MustInvoke[db.DB](i)
	jwks := do.MustInvoke[jwt.Jwks](i)
	// db.DB().AutoMigrate(&Users{})

	return &usersService{
		db:   db,
		jwks: jwks,
	}, nil
}

func (u *usersService) GetUser(id uuid.UUID) (*Users, error) {
	users := u.load()

	for _, user := range users {
		if user.ID == id {
			return &user, nil
		}
	}

	return nil, errors.New("user not found")
}

func (u *usersService) CreateUser(user *Users) error {

	if err := u.save(user); err != nil {
		return err
	}

	return nil
}

func (u *usersService) UpdateUser(token string, user *Users) error {
	id, err := u.jwks.ValidateToken(token)
	if err != nil {
		return err
	}

	if err := u.update(uuid.FromStringOrNil(id), user); err != nil {
		return err
	}

	return nil
}

func (u *usersService) DeleteUser(token string) error {
	user, err := u.GetMe(token)
	if err != nil {
		return err
	}

	if err := u.delete(user); err != nil {
		return err
	}

	return nil
}

func (u *usersService) GetMe(token string) (*Users, error) {
	id, err := u.jwks.ValidateToken(token)
	if err != nil {
		return nil, err
	}

	return u.GetUser(uuid.FromStringOrNil(id))
}

func (u *usersService) LoginUser(email string, password string) (string, error) {
	users := u.load()

	for _, user := range users {
		if user.Email == email && user.Password == password {

			token, err := u.jwks.GenerateToken(user.ID.String())
			if err != nil {
				return "", err
			}

			return token, nil
		}
	}

	return "", errors.New("user not found")
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
	result := db.Model(&Users{}).Where(db.Where("email = ?", user.Email).Or("wallet_address = ?", user.WalletAddress)).First(&existingUser)
	if result.RowsAffected == 1 {
		return errors.New("user already exists")
	}

	log.Printf("user: %+v", existingUser)

	if user.Role != Admin && user.Role != Customer && user.Role != Seller {
		return errors.New("invalid role")
	}

	result = db.Save(&user)
	if result.Error != nil {
		return result.Error
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
