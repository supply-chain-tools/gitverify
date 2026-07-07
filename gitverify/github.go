package gitverify

const (
	gitHubForgeId = "github.com"
	gitHubEmail   = "noreply@github.com"

	gitlabForgeId = "gitlab.com"
	gitlabEmail   = "noreply@gitlab.com"
)

func gitHubUserEmail(userId string, username string) string {
	return userId + "+" + username + "@users.noreply.github.com"
}
