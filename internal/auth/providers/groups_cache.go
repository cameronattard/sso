package providers

import (
	"reflect"
	"time"

	"github.com/buzzfeed/sso/internal/pkg/groups"
	"github.com/buzzfeed/sso/internal/pkg/sessions"
	"github.com/datadog/datadog-go/statsd"
)

var (
	// This is a compile-time check to make sure our types correctly implement the interface:
	// https://medium.com/@matryer/golang-tip-compile-time-checks-to-ensure-your-type-satisfies-an-interface-c167afed3aae
	_ Provider = &GroupCache{}
)

type Cache interface {
	Get(keys string)
	Set(entries []string)
	Purge(keys []string)
}

type GroupCache struct {
	metrics  *statsd.Client
	provider Provider
	cache    *groups.LocalCache
}

func NewLocalCache(provider Provider, ttl time.Duration, statsdClient *statsd.Client, tags []string) *GroupCache {
	return &GroupCache{
		metrics:  statsdClient,
		provider: provider,
		cache:    groups.NewLocalCache(ttl, statsdClient, tags),
	}
}

func (p *GroupCache) Data() *ProviderData {
	return p.provider.Data()
}

func (p *GroupCache) Redeem(redirectURL, code string) (*sessions.SessionState, error) {
	return p.provider.Redeem(redirectURL, code)
}

func (p *GroupCache) ValidateSessionState(s *sessions.SessionState) bool {
	return p.provider.ValidateSessionState(s)
}

func (p *GroupCache) GetSignInURL(redirectURI, finalRedirect string) string {
	return p.provider.GetSignInURL(redirectURI, finalRedirect)
}

func (p *GroupCache) RefreshSessionIfNeeded(s *sessions.SessionState) (bool, error) {
	return p.provider.RefreshSessionIfNeeded(s)
}

// ValidateGroupMembership wraps the provider's ValidateGroupMembership around calls to check local cache for group membership information.
func (p *GroupCache) ValidateGroupMembership(email string, allowedGroups []string, accessToken string) ([]string, error) {
	groupMembership, err := p.cache.Get(email)
	if reflect.DeepEqual(groupMembership.UserGroupData.AllowedGroups, allowedGroups) && len(groupMembership.UserGroupData.MatchedGroups) > 0 {
		// If the passed in allowed groups match the cached version, and the length of the cached 'matched' groups are greater than zero,
		// return the cached groups
		return groupMembership.UserGroupData.MatchedGroups, nil
	}

	// If the user's group membership is not in cache, or the passed list of 'AllowedGroups'
	// differs from the cached entry, call and return the groups from p.Provider.ValidateGroupMembership.
	validGroups, err := p.provider.ValidateGroupMembership(email, allowedGroups, accessToken)
	if err != nil {
		return nil, err
	}

	// Create and cache an Entry and return the valid groups.
	entry := groups.Entry{
		Key: email,
		UserGroupData: groups.UserGroupData{
			AllowedGroups: allowedGroups,
			MatchedGroups: validGroups,
		},
	}
	_, err = p.cache.Set(entry)
	if err != nil {
	}
	return validGroups, nil
}

func (p *GroupCache) Revoke(s *sessions.SessionState) error {
	return p.provider.Revoke(s)
}

func (p *GroupCache) RefreshAccessToken(refreshToken string) (string, time.Duration, error) {
	return p.provider.RefreshAccessToken(refreshToken)
}

func (p *GroupCache) Stop() {
	p.provider.Stop()
}
