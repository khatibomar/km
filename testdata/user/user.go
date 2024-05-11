package user

import "time"

type User struct {
	Name     string
	Age      int
	MetaData Metadata
}

type Metadata struct {
	CreatedAt time.Time
}
