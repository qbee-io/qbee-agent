package configuration

// Users adds or removes users.
//
// Example payload:
// {
//  "items": [
//    {
//      "username": "test",
//      "action": "remove"
//    }
//  ]
// }
type Users struct {
	Metadata

	Users []User `json:"items"`
}

// UserAction defines what to do with a user.
type UserAction string

const (
	UserAdd    UserAction = "add"
	UserRemove UserAction = "remove"
)

// User defines a user to be modified in the system.
type User struct {
	Username string     `json:"username"`
	Action   UserAction `json:"action"`
}
