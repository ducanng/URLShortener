package models

type User struct {
	id       int
	username string
	password string
	email    string
}

func (u *User) Id() int {
	return u.id
}

func (u *User) SetId(id int) {
	u.id = id
}

func (u *User) Username() string {
	return u.username
}

func (u *User) SetUsername(username string) {
	u.username = username
}

func (u *User) Password() string {
	return u.password
}

func (u *User) SetPassword(password string) {
	u.password = password
}

func (u *User) Email() string {
	return u.email
}

func (u *User) SetEmail(email string) {
	u.email = email
}
