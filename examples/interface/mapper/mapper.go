package mapper

import (
	"github.com/khatibomar/km/examples/interface/domain"
	"github.com/khatibomar/km/examples/interface/dto"
)

//go:generate go run ../../../main.go ../../../generator.go -type UserMapper

type UserMapper interface {
	MapUser(in domain.User) dto.UserDTO
}
