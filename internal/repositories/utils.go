package repositories

import (
	"errors"

	"gorm.io/gorm"
)

func IsNotFoundError(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
