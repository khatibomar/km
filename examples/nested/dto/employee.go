package dto

type AddressDTO struct {
	Street  string
	City    string
	ZipCode int64
}
type ContactDTO struct {
	Type  string
	Value string
}
type EmployeeDTO struct {
	ID        int64
	FirstName string
	LastName  string
	Address   *AddressDTO
	Contacts  []ContactDTO
}
