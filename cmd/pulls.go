package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/go-github/v48/github"
	"github.com/rasecoiac03/github-rankings/common"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

var (
	pullsCmd = &cobra.Command{
		Use:   "pulls",
		Short: "Get pull requests ranking",
		Long:  `Get pull requests ranking`,
		RunE:  pullsExecute,
	}

	aDay = 1 * 24 * time.Hour

	org            string
	user           string
	dateRange      string
	includeReviews bool
)

func init() {
	pullsCmd.Flags().StringVar(&org, "org", "", "Github organization")
	pullsCmd.Flags().StringVar(&user, "user", "", "username")
	pullsCmd.Flags().StringVar(&dateRange, "date-range", "", "Date range, format: yyyy-MM-dd..yyyy-MM-dd")
	pullsCmd.Flags().BoolVar(&includeReviews, "get-reviews", false, "")
	rootCmd.AddCommand(pullsCmd)
}

func pullsExecute(cmd *cobra.Command, args []string) (err error) {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: common.GetEnv("GH_TOKEN")},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	pullRequests := []*github.Issue{}

	if dateRange != "" {
		var dateFrom time.Time
		var dateTo time.Time
		dateRangeSplit := strings.Split(dateRange, "..")
		dateFrom, err = time.Parse("2006-01-02", dateRangeSplit[0])
		if err != nil {
			return
		}

		dateTo, err = time.Parse("2006-01-02", dateRangeSplit[1])
		if err != nil {
			return
		}

		controlProcessedDate := map[string]struct{}{}

		for {
			if dateFrom.After(dateTo) {
				break
			}

			var ps []*github.Issue
			dateQuery := dateFrom.Format("2006-01-02")
			if _, present := controlProcessedDate[dateQuery]; present {
				logrus.Debugf("already processed date %s", dateQuery)
				continue
			}

			controlProcessedDate[dateQuery] = struct{}{}
			ps, err = getPullRequests(ctx, client, org, dateQuery)
			if err != nil {
				return
			}
			pullRequests = append(pullRequests, ps...)

			dateFrom = dateFrom.Add(aDay)
		}
	} else {
		var ps []*github.Issue
		ps, err = getPullRequests(ctx, client, org, "")
		if err != nil {
			return
		}
		pullRequests = append(pullRequests, ps...)
	}

	userPullRequestsCount := map[string]int{}
	relatedRepos := map[string]struct{}{}

	for _, p := range pullRequests {
		if p.User.Login == nil {
			continue
		}

		userPullRequestsCount[*p.User.Login]++

		repo := strings.ReplaceAll(*p.RepositoryURL, "https://api.github.com/repos/"+org+"/", "")
		relatedRepos[repo] = struct{}{}

		if includeReviews {
			var reviews []*github.PullRequestReview
			reviews, err = getPullRequestReviews(ctx, client, org, repo, *p.Number)
			if err != nil {
				return
			}

			for _, review := range reviews {
				if review.User.Login != nil {
					logrus.Debug("reviewed by", *review.User.Login)
				}
			}
		}
	}

	users := make([]string, 0, len(userPullRequestsCount))
	for key := range userPullRequestsCount {
		users = append(users, key)
	}
	repos := []string{}
	for repo := range relatedRepos {
		repos = append(repos, repo)
	}

	sort.SliceStable(users, func(i, j int) bool {
		return userPullRequestsCount[users[i]] > userPullRequestsCount[users[j]]
	})

	markdownRanking := "| user | count |\n| ---- | ----- |"
	for _, user := range users {
		logrus.Debugf("user %s, count %d", user, userPullRequestsCount[user])
		markdownRanking += fmt.Sprintf("\n| %s | %d |", user, userPullRequestsCount[user])
	}

	logrus.Debugf("markdown ranking:\n%s", markdownRanking)
	logrus.Debugf("related repos:\n%s", strings.Join(repos, "\n"))

	return
}

func getPullRequests(ctx context.Context, client *github.Client,
	org, dateQuery string) (pullRequests []*github.Issue, err error) {
	page := 1
	for {
		var ps *github.IssuesSearchResult
		var resp *github.Response

		query := fmt.Sprintf("NOT Snyk is:pr archived:false org:%s", org)
		if dateQuery != "" {
			query += " created:" + dateQuery
		}
		if user != "" {
			query += " author:" + user
		}

		logrus.Debugf("get pull requests query: %s", query)

		ps, resp, err = client.Search.Issues(ctx, query,
			&github.SearchOptions{
				ListOptions: github.ListOptions{
					Page:    page,
					PerPage: 100, // default 30, max 100
				},
			})
		if err != nil {
			if rateLimitErr, ok := err.(*github.RateLimitError); ok {
				diff := time.Since(rateLimitErr.Rate.Reset.Time)
				logrus.Debugf("rate limit caught us, reset: %s, now - reset: %s", rateLimitErr.Rate.Reset.Time, diff)
				time.Sleep(diff)
				continue
			}

			if abuseRateLimitErr, ok := err.(*github.AbuseRateLimitError); ok {
				logrus.Debugf("abuse rate limit caught us, retry after: %s", abuseRateLimitErr.RetryAfter)
				time.Sleep(30 * time.Second)
				continue
			}

			return
		}

		pullRequests = append(pullRequests, ps.Issues...)

		if resp.NextPage <= 0 {
			break
		}

		page = resp.NextPage
	}

	return
}

func getPullRequestReviews(ctx context.Context, client *github.Client,
	org, repo string, number int) (reviews []*github.PullRequestReview, err error) {
	reviews, _, err = client.PullRequests.ListReviews(ctx, org, repo, number, &github.ListOptions{
		PerPage: 100,
	})
	if err != nil {
		if rateLimitErr, ok := err.(*github.RateLimitError); ok {
			diff := time.Since(rateLimitErr.Rate.Reset.Time)
			logrus.Debugf("rate limit caught us, reset: %s, now - reset: %s", rateLimitErr.Rate.Reset.Time, diff)
			time.Sleep(diff)
			return getPullRequestReviews(ctx, client, org, repo, number)
		}

		if abuseRateLimitErr, ok := err.(*github.AbuseRateLimitError); ok {
			logrus.Debugf("abuse rate limit caught us, retry after: %s", abuseRateLimitErr.RetryAfter)
			time.Sleep(30 * time.Second)
			return getPullRequestReviews(ctx, client, org, repo, number)
		}

		return
	}

	return
}
