package configuration

// Password bundle sets passwords for existing users.
//
// Example payload:
// {
//  "users": [
//    {
//      "username": "piotr",
//      "passwordhash": "$6$EMNbdq1ZkOAZSpFt$t6Ei4J11Ybip1A51sbBPTtQEVcFPPPUs.Q9nle4FenvrId4fLr8douwE3lbgWZGK.LIPeVmmFrTxYJ0QoYkFT."
//    }
//  ]
// }
type Password struct {
	Metadata

	Users []UserPassword `json:"users"`
}

type UserPassword struct {
	Username     string `json:"username"`
	PasswordHash string `json:"passwordhash"`
}
