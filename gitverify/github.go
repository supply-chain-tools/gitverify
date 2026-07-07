package gitverify

const (
	gitHubForgeId = "github.com"
	gitHubEmail   = "noreply@github.com"
)

func gitHubUserEmail(userId string, username string) string {
	return userId + "+" + username + "@users.noreply.github.com"
}
