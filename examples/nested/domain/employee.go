package domain

type Address struct {
	Street  string
	City    string
	ZipCode int
}
type Contact struct {
	Type  string
	Value string
}
type Employee struct {
	ID        int
	FirstName string
	LastName  string
	Address   *Address
	Contacts  []Contact
}
