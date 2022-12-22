//go:build linux

package inventory

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
)

const (
	passwdFilePath = "/etc/passwd"
	shadowFilePath = "/etc/shadow"
)

const (
	PasswordAlgorithmMD5      = 1
	PasswordAlgorithmBcrypt   = 2
	PasswordAlgorithmSHA1     = 4
	PasswordAlgorithmSHA256   = 5
	PasswordAlgorithmSHA512   = 6
	PasswordAlgorithmYesCrypt = 7
)

var shadowAlgorithms = map[string]int{
	"1":    PasswordAlgorithmMD5,
	"2":    PasswordAlgorithmBcrypt,
	"2a":   PasswordAlgorithmBcrypt,
	"2b":   PasswordAlgorithmBcrypt,
	"2x":   PasswordAlgorithmBcrypt,
	"2y":   PasswordAlgorithmBcrypt,
	"sha1": PasswordAlgorithmSHA1,
	"5":    PasswordAlgorithmSHA256,
	"6":    PasswordAlgorithmSHA512,
	"7":    PasswordAlgorithmYesCrypt,
	"y":    PasswordAlgorithmYesCrypt,
}

// CollectUsersInventory returns populated Users inventory based on current system status.
func CollectUsersInventory() (*Users, error) {
	users, err := getUsersFromPasswd(passwdFilePath, shadowFilePath)
	if err != nil {
		return nil, err
	}

	usersInventory := &Users{
		Users: users,
	}

	return usersInventory, nil
}

// getUsersFromPasswd returns users based on passwd file.
func getUsersFromPasswd(passwdFilePath, shadowFilePath string) ([]User, error) {
	// get mapping of username -> User (with populated password fields)
	usersPasswords, err := getUsersFromShadow(shadowFilePath)
	if err != nil {
		return nil, err
	}

	var passwdFile *os.File
	passwdFile, err = os.Open(passwdFilePath)
	if err != nil {
		return nil, fmt.Errorf("error openning %s: %w", passwdFilePath, err)
	}

	defer passwdFile.Close()

	scanner := bufio.NewScanner(passwdFile)

	users := make([]User, 0)
	for scanner.Scan() {
		if err = scanner.Err(); err != nil {
			return nil, fmt.Errorf("error reading %s: %w", passwdFilePath, err)
		}

		fields := strings.Split(scanner.Text(), ":")

		var uid, gid int
		if uid, err = strconv.Atoi(fields[2]); err != nil {
			return nil, fmt.Errorf("invalid UID for user %s in %s", fields[0], passwdFilePath)
		}

		if gid, err = strconv.Atoi(fields[3]); err != nil {
			return nil, fmt.Errorf("invalid GID for user %s in %s", fields[0], passwdFilePath)
		}

		user := User{
			Name:          fields[0],
			UID:           uid,
			GID:           gid,
			GECOS:         fields[4],
			HomeDirectory: fields[5],
			Shell:         fields[6],
			HasPassword:   "no",
		}

		// check if password is specified in the passwd file
		password := fields[1]
		if password != "x" && password != "*" && password != "" {
			user.HasPassword = "yes"
		}

		// passwords from shadow take precedence
		userPassword, ok := usersPasswords[user.Name]
		if ok {
			user.HasPassword = userPassword.HasPassword
			user.PasswordAlgorithm = userPassword.PasswordAlgorithm
			user.PasswordAge = userPassword.PasswordAge
		}

		users = append(users, user)
	}

	return users, nil
}

// getUsersFromShadow returns map of users with password related fields populated.
func getUsersFromShadow(filePath string) (map[string]User, error) {
	users := make(map[string]User)

	shadowFile, err := os.Open(filePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return users, nil
		}

		return nil, fmt.Errorf("error openning %s: %w", filePath, err)
	}

	defer shadowFile.Close()

	scanner := bufio.NewScanner(shadowFile)

	for scanner.Scan() {
		if err = scanner.Err(); err != nil {
			return nil, fmt.Errorf("error reading %s: %w", filePath, err)
		}

		fields := strings.Split(scanner.Text(), ":")

		// skip invalid passwords
		passwordFields := strings.Split(fields[1], "$")
		if len(passwordFields) == 1 {
			continue
		}

		var age int
		if age, err = strconv.Atoi(fields[2]); err != nil {
			return nil, fmt.Errorf("invalid passowrd age for user %s in %s", fields[0], filePath)
		}

		users[fields[0]] = User{
			HasPassword:       "yes",
			PasswordAlgorithm: shadowAlgorithms[passwordFields[1]],
			PasswordAge:       age,
		}
	}

	return users, nil
}
