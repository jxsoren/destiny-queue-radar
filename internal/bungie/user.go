package bungie

import (
	"context"
	"errors"
	"net/http"
)

var ErrNoDestinyAccount = errors.New("bungie: account has no Destiny 2 membership")

// CurrentUser is the signed-in Bungie.net account and its primary Destiny membership.
type CurrentUser struct {
	BungieNetMembershipID string
	BungieName            string // "Name#1234"
	Destiny               UserInfoCard
}

type membershipsResponse struct {
	DestinyMemberships  []UserInfoCard `json:"destinyMemberships"`
	PrimaryMembershipID string         `json:"primaryMembershipId"`
	BungieNetUser       struct {
		MembershipID string `json:"membershipId"`
		UniqueName   string `json:"uniqueName"`
	} `json:"bungieNetUser"`
}

// GetCurrentUser identifies the owner of an OAuth access token.
func (c *Client) GetCurrentUser(ctx context.Context, accessToken string) (CurrentUser, error) {
	m, err := callAs[membershipsResponse](ctx, c, accessToken, http.MethodGet,
		"/Platform/User/GetMembershipsForCurrentUser/", nil)
	if err != nil {
		return CurrentUser{}, err
	}
	if len(m.DestinyMemberships) == 0 {
		return CurrentUser{}, ErrNoDestinyAccount
	}
	card := crossSavePrimary(m.DestinyMemberships)
	for _, cd := range m.DestinyMemberships {
		if m.PrimaryMembershipID != "" && cd.MembershipID == m.PrimaryMembershipID {
			card = cd
			break
		}
	}
	name := m.BungieNetUser.UniqueName
	if card.BungieGlobalDisplayName != "" {
		name = FormatBungieName(card.BungieGlobalDisplayName, card.BungieGlobalDisplayNameCode)
	}
	return CurrentUser{BungieNetMembershipID: m.BungieNetUser.MembershipID, BungieName: name, Destiny: card}, nil
}
