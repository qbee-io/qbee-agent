package inventory

const TypeUsers Type = "users"

type Users struct {
	Users []User `json:"items"`
}

// GetUser returns User with the provided username or nil if user does not exist in the system.
func (users Users) GetUser(username string) *User {
	for i := range users.Users {
		if users.Users[i].Name == username {
			return &users.Users[i]
		}
	}

	return nil
}

type User struct {
	// Name - the string a user would type in when logging into the operating system.
	Name string `json:"user"`

	// UID - user identifier number.
	UID int `json:"uid"`

	// GID - group identifier number, which identifies the primary group of the user.
	GID int `json:"gid"`

	// GECOS - general information about the user, such as their real name and phone number.
	GECOS string `json:"gecos"`

	// HomeDirectory - path to the user's home directory.
	HomeDirectory string `json:"home"`

	// Shell - program that is started every time the user logs into the system.
	Shell string `json:"shell"`

	// HasPassword - "yes" if user has password set.
	HasPassword string `json:"has_pwd"`

	// PasswordAlgorithm
	// Probably should be fixed to return one of the following:
	//    $1$ – MD5
	//    $2$, $2a$, $2b$ – bcrypt
	//    $5$ – SHA-256
	//    $6$ – SHA-512
	//    $y$ – yescrypt
	// Example from /etc/shadow:
	// vm:$y$j9T$zlG11k7j50csbROp/ZF430$.xYApDc/8FH2T9qvGntFS9IxmzK2F4gBYFe/8EgUba6:19305:0:99999:7:::
	// ^^ is using yescrypt
	PasswordAlgorithm int `json:"pwd_alg"`

	// PasswordAge - days since epoch of last password change.
	PasswordAge int `json:"pwd_age"`
}
