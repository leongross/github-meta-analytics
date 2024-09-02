package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/google/go-github/v64/github"
)

// CompanyStats is a struct that holds the name of the organization and the contributors
// of the organization
type CompanyStats struct {
	Name          string
	Contributors  []github.Contributor
	Contributions int
}

func (c *CompanyStats) String() string {
	return fmt.Sprintf("%s, Contributors: %v, Contributions: %d", c.Name, func() []string {
		var contributors []string
		for _, contributor := range c.Contributors {
			contributors = append(contributors, contributor.GetLogin())
		}
		return contributors
	}(), c.Contributions)
}

func ParseCompanyStats(stats []*github.ContributorStats, client *github.Client, ctx context.Context) (orgStats map[string]CompanyStats, err error) {
	companies := make(map[string]CompanyStats)
	for _, stat := range stats {
		contributor := stat.GetAuthor()
		username := contributor.GetLogin()

		// use search service to get the matching user to login data
		user, _, err := client.Users.Get(ctx, username)
		if err != nil {
			return nil, err
		}

		// get the company field of the user
		company := user.GetCompany()
		if len(company) == 0 {
			company = "No company"
		} else if company[0] == '@' {
			company = company[1:]
		}

		// get orgs the user is affiliated with
		// orgs, _, err := client.Organizations.List(ctx, username, nil)
		val, ok := companies[company]
		if !ok {
			// company not in map, add it
			companies[company] = CompanyStats{
				Name:          company,
				Contributors:  []github.Contributor{*contributor},
				Contributions: *stat.Total,
			}
		} else {
			// company is in map, update the values
			val.Contributors = append(val.Contributors, *contributor)
			val.Contributions += *stat.Total
			companies[company] = val
		}
	}

	return companies, nil
}

// Prase github URL and return
// TODO: wrap Repo and Author in a struct and also return an error
func ParseUrl(url string) (authors string, repo string) {
	tokens := strings.Split(url, "/")

	switch len(tokens) {
	// github.com/author/repo
	case 3:
		return tokens[1], tokens[2]

	// https://github.com/authors/repo
	case 5:
		return tokens[3], tokens[4]
	default:
		log.Fatalf("invalid url")
	}
	return "", ""
}

func main() {
	var (
		client *github.Client
		stats  []*github.ContributorStats
		err    error
	)

	if len(os.Args) != 2 {
		log.Fatalf("no url specified")
	}

	// parse url for format {https://}+github.com/<author>/repo
	author, repo := ParseUrl(os.Args[1])

	// for now only use the default client since we want read only access on public repos
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		log.Printf("No GITHUB_TOKEN found in environment, you might get rate limited!")
		client = github.NewClient(nil)
	} else {
		log.Printf("Using token for GitHub API access")
		client = github.NewClient(nil).WithAuthToken(token)
	}
	ctx := context.Background()

	for {
		stats, _, err = client.Repositories.ListContributorsStats(ctx, author, repo)
		if _, ok := err.(*github.AcceptedError); ok {
			log.Println("scheduled on GitHub side")
			time.Sleep(3 * time.Second)
			continue
		} else {
			break
		}
	}

	compStats, err := ParseCompanyStats(stats, client, ctx)
	if err != nil {
		log.Fatal(err)
	}

	// sort the companies by contributions
	sortedCompanies := make([]CompanyStats, 0)
	for _, val := range compStats {
		sortedCompanies = append(sortedCompanies, val)
	}

	sort.Slice(sortedCompanies, func(i, j int) bool {
		return sortedCompanies[i].Contributions > sortedCompanies[j].Contributions
	})

	fmt.Printf("Company stats for %s/%s\n", author, repo)
	fmt.Printf("Total contributors: %d\n", len(stats))
	fmt.Printf("Total companies: %d\n\n", len(compStats))
	fmt.Println("Companies sorted by contributions:")

	for _, val := range sortedCompanies {
		fmt.Println(val.String())
	}
}
