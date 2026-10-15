package bungie

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type UserInfoCard struct {
	MembershipType              int64  `json:"membershipType"`
	MembershipID                string `json:"membershipId"`
	CrossSaveOverride           int64  `json:"crossSaveOverride"`
	BungieGlobalDisplayName     string `json:"bungieGlobalDisplayName"`
	BungieGlobalDisplayNameCode int    `json:"bungieGlobalDisplayNameCode"`
}

// ParseBungieName splits "Name#1234" into its display name and 4-digit code.
func ParseBungieName(s string) (string, int, error) {
	s = strings.TrimSpace(s)
	i := strings.LastIndex(s, "#")
	if i <= 0 {
		return "", 0, fmt.Errorf("bungie name %q: expected Name#1234", s)
	}
	name, codeStr := s[:i], s[i+1:]
	if len(codeStr) != 4 {
		return "", 0, fmt.Errorf("bungie name %q: code must be 4 digits", s)
	}
	for _, c := range codeStr {
		if c < '0' || c > '9' {
			return "", 0, fmt.Errorf("bungie name %q: code must be numeric", s)
		}
	}
	code, err := strconv.Atoi(codeStr)
	if err != nil {
		return "", 0, fmt.Errorf("bungie name %q: code must be numeric", s)
	}
	return name, code, nil
}

func FormatBungieName(name string, code int) string {
	return fmt.Sprintf("%s#%04d", name, code)
}

type searchBody struct {
	DisplayName     string `json:"displayName"`
	DisplayNameCode int    `json:"displayNameCode"`
}

// SearchByBungieName resolves a Bungie Name to the player's primary (Cross Save) membership.
func (c *Client) SearchByBungieName(ctx context.Context, name string, code int) (UserInfoCard, error) {
	cards, err := call[[]UserInfoCard](ctx, c, http.MethodPost,
		"/Platform/Destiny2/SearchDestinyPlayerByBungieName/-1/", searchBody{DisplayName: name, DisplayNameCode: code})
	if err != nil {
		return UserInfoCard{}, err
	}
	if len(cards) == 0 {
		return UserInfoCard{}, fmt.Errorf("%s: %w", FormatBungieName(name, code), ErrPlayerNotFound)
	}
	for _, card := range cards {
		if card.CrossSaveOverride != 0 && card.MembershipType == card.CrossSaveOverride {
			return card, nil
		}
	}
	return cards[0], nil
}
