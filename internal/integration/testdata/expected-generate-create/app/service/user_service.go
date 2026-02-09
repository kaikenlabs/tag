package service

// UserService handles business logic.
type UserService struct {
	name string
}

// NewUserService creates a new UserService.
func NewUserService(name string) *UserService {
	return &UserService{name: name}
}
