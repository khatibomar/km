package dto

import (
	"github.com/khatibomar/km/testdata/user"
)

type UserDTO struct {
	Name     string
	Age      int
	MetaData user.Metadata
}
