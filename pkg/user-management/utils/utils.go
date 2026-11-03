package utils

import (
	"bufio"
	"log/slog"
	"net/mail"
	"os"
	"regexp"
	"strings"
)

const (
	PASSWORD_MIN_LEN = 12
	PASSWORD_MAX_LEN = 512
)

var blockedPasswords map[string]struct{}

func LoadBlockedPasswords(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	blockedPasswords = make(map[string]struct{})
	scanner := bufio.NewScanner(file)
	lines := 0
	usedEntries := 0
	for scanner.Scan() {
		lines += 1
		passwordEntry := scanner.Text()
		passwordEntry = strings.TrimSpace(passwordEntry)
		passwordEntry = strings.Trim(passwordEntry, "\n")
		if CheckPasswordFormat(passwordEntry) {
			usedEntries += 1
			blockedPasswords[scanner.Text()] = struct{}{}
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}
	slog.Info("loaded blocked password list", slog.Int("lines", lines), slog.Int("used", usedEntries))
	return nil
}

func SanitizeEmail(email string) string {
	email = strings.ToLower(email)
	email = strings.Trim(email, " \n\r")
	return email
}

func SanitizePhoneNumber(phone string) string {
	phone = strings.ToLower(phone)
	phone = strings.Trim(phone, " \n\r")
	return phone
}

// CheckEmailFormat to check if input string is a correct email address
func CheckEmailFormat(email string) bool {
	if len(email) > 254 {
		return false
	}
	_, err := mail.ParseAddress(email)
	if err != nil {
		return false
	}
	// additional regex check for correct email format
	emailRule := regexp.MustCompile(`^[a-zA-Z0-9._%+'-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRule.MatchString(email)
}

// BlurEmailAddress transforms an email address to reduce exposed personal info
func BlurEmailAddress(email string) string {
	items := strings.Split(email, "@")
	if len(items) < 1 || len(items[0]) < 1 {
		return "****@**"
	}

	blurredEmail := string([]rune(items[0])[0]) + "****@" + strings.Join(items[1:], "")
	return blurredEmail
}

// CheckPasswordFormat to check if password fulfills password rules
func CheckPasswordFormat(password string) bool {
	// Count UTF-16 code units, matching JavaScript string.length.
	pl := 0
	for _, r := range password {
		pl++
		if r > 0xFFFF {
			pl++
		}
		if pl > PASSWORD_MAX_LEN {
			return false
		}
	}
	if pl < PASSWORD_MIN_LEN {
		return false
	}

	var res = 0

	lowercase := regexp.MustCompile("[a-z]")
	uppercase := regexp.MustCompile("[A-Z]")
	number := regexp.MustCompile(`\d`) //"^(?:(?=.*[a-z])(?:(?=.*[A-Z])(?=.*[\\d\\W])|(?=.*\\W)(?=.*\d))|(?=.*\W)(?=.*[A-Z])(?=.*\d)).{8,}$")
	symbol := regexp.MustCompile(`\W`)

	if lowercase.MatchString(password) {
		res++
	}
	if uppercase.MatchString(password) {
		res++
	}
	if number.MatchString(password) {
		res++
	}
	if symbol.MatchString(password) {
		res++
	}
	return res > 2
}

func IsPasswordOnBlocklist(password string) bool {
	_, exists := blockedPasswords[password]
	return exists
}

// CheckLanguageCode checks if a string can be considered as a language code
func CheckLanguageCode(code string) bool {
	codeRule := regexp.MustCompile(`^[a-z]{2}(-([A-Z]{2}|\d{3}))?$`)
	return codeRule.MatchString(code)
}
