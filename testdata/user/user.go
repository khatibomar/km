package user

import "time"

type User struct {
	Name      string
	Age       int
	MetadData Metadata
}

type Metadata struct {
	CreatedAt time.Time
}
