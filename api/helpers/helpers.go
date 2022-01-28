package helpers

import (
	"os"
	"strings"
)

func CheckIfUrlContainsDomain(url string) bool {
	domain := os.Getenv("DOMAIN")
	return strings.Contains(url, domain)
}