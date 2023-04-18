package test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const mailcatcherDefaultBaseURL = "http://localhost:1080"

var mailcatcherBaseURL = os.Getenv("QBEE_MAILCATCHER_BASE_URL")

func init() {
	if mailcatcherBaseURL == "" {
		mailcatcherBaseURL = mailcatcherDefaultBaseURL
	}
}

type EmailMessages []EmailMessage

type EmailMessage struct {
	ID         int      `json:"id"`
	Recipients []string `json:"recipients"`
	Subject    string   `json:"subject"`
}

// GetEmailMessages returns captured tet email messages for provided recipient email.
func GetEmailMessages(email string) EmailMessages {
	// get all the messages from the mailcatcher
	resp, err := http.DefaultClient.Get(mailcatcherBaseURL + "/messages")
	if err != nil {
		panic(err)
	}

	defer resp.Body.Close()

	allMessages := make(EmailMessages, 0)
	if err = json.NewDecoder(resp.Body).Decode(&allMessages); err != nil {
		panic(err)
	}

	// filter out only messages where any of the recipients matched provided email
	expectedRecipient := fmt.Sprintf("<%s>", email)

	messages := make(EmailMessages, 0, len(allMessages))

	for i := range allMessages {
		for _, recipient := range allMessages[i].Recipients {
			if strings.EqualFold(recipient, expectedRecipient) {
				messages = append(messages, allMessages[i])
			}
		}
	}

	return messages
}

func GetPlainMessage(id int) string {
	url := fmt.Sprintf("%s/messages/%d.plain", mailcatcherBaseURL, id)

	resp, err := http.DefaultClient.Get(url)
	if err != nil {
		panic(err)
	}

	defer resp.Body.Close()

	emailBodyBytes, _ := io.ReadAll(resp.Body)

	return string(emailBodyBytes)
}
