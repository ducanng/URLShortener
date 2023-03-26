package models

type User struct {
	Id       int    `json:"id"`
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
}

func (u *User) GetId() int {
	return u.Id
}

func (u *User) SetId(id int) {
	u.Id = id
}

func (u *User) GetUsername() string {
	return u.Username
}

func (u *User) SetUsername(username string) {
	u.Username = username
}

func (u *User) GetPassword() string {
	return u.Password
}

func (u *User) SetPassword(password string) {
	u.Password = password
}

func (u *User) GetEmail() string {
	return u.Email
}

func (u *User) SetEmail(email string) {
	u.Email = email
}
