package configuration

// SSHKeys adds or removes authorized SSH keys for users.
//
// Example payload:
// {
//  "users": [
//    {
//      "username": "test",
//      "userkeys": [
//        "key1",
//        "key2"
//      ]
//    }
//  ]
// }
type SSHKeys struct {
	Metadata

	Users []SSHKey `json:"users"`
}

// SSHKey defines an SSH key to be added to a user.
type SSHKey struct {
	Username string   `json:"username"`
	Keys     []string `json:"userkeys"`
}
