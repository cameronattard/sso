package providers

import (
	"reflect"
	"sort"
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
	Get(keys string) (groups.Entry, error)
	Set(entries []string) (groups.Entry, error)
	Purge(keys []string) error
}

// GroupCache is designed to act as a provider while wrapping subsequent provider's functions,
// while also offering a caching mechanism (specifically used for group caching at the moment).
type GroupCache struct {
	StatsdClient *statsd.Client
	provider     Provider
	cache        *groups.LocalCache
}

// NewLocalCache returns a new GroupCache (which includes a LocalCache from the groups package)
func NewLocalCache(provider Provider, ttl time.Duration, statsdClient *statsd.Client, tags []string) *GroupCache {
	return &GroupCache{
		StatsdClient: statsdClient,
		provider:     provider,
		cache:        groups.NewLocalCache(ttl, statsdClient, tags),
	}
}

// SetStatsdClient calls the provider's SetStatsdClient function.
func (p *GroupCache) SetStatsdClient(StatsdClient *statsd.Client) {
	p.StatsdClient = StatsdClient
	p.provider.SetStatsdClient(StatsdClient)
}

// Data returns the provider Data
func (p *GroupCache) Data() *ProviderData {
	return p.provider.Data()
}

// Redeem wraps the provider's Redeem function
func (p *GroupCache) Redeem(redirectURL, code string) (*sessions.SessionState, error) {
	return p.provider.Redeem(redirectURL, code)
}

// ValidateSessionState wraps the provider's ValidateSessionState function.
func (p *GroupCache) ValidateSessionState(s *sessions.SessionState) bool {
	return p.provider.ValidateSessionState(s)
}

// GetSignInURL wraps the provider's GetSignInURL function.
func (p *GroupCache) GetSignInURL(redirectURI, finalRedirect string) string {
	return p.provider.GetSignInURL(redirectURI, finalRedirect)
}

// RefreshSessionIfNeeded wraps the provider's RefreshSessionIfNeeded function.
func (p *GroupCache) RefreshSessionIfNeeded(s *sessions.SessionState) (bool, error) {
	return p.provider.RefreshSessionIfNeeded(s)
}

// ValidateGroupMembership wraps the provider's ValidateGroupMembership around calls to check local cache for group membership information.
func (p *GroupCache) ValidateGroupMembership(email string, allowedGroups []string, accessToken string) ([]string, error) {
	groupMembership, err := p.cache.Get(email)
	if err != nil {
		return nil, err
	}
	// If the passed in allowed groups match the cached version, and the length of the cached 'matched' groups are greater than zero,
	// return the cached groups
	if reflect.DeepEqual(groupMembership.UserGroupData.AllowedGroups, allowedGroups) {
		if len(groupMembership.UserGroupData.MatchedGroups) > 0 {
			p.StatsdClient.Incr("provider.groupcache",
				[]string{
					"action:ValidateGroupMembership",
					"cache:hit",
				}, 1.0)
			return groupMembership.UserGroupData.MatchedGroups, nil
		}
	}
	p.StatsdClient.Incr("provider.groupcache",
		[]string{
			"action:ValidateGroupMembership",
			"cache:miss",
		}, 1.0)

	// If the user's group membership is not in cache, or the passed list of 'AllowedGroups'
	// differs from the cached entry, call and return the groups from p.Provider.ValidateGroupMembership.
	validGroups, err := p.provider.ValidateGroupMembership(email, allowedGroups, accessToken)
	if err != nil {
		return nil, err
	}

	// Create and cache an Entry and return the valid groups.
	sort.Strings(allowedGroups)
	sort.Strings(validGroups)
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

// Revoke wraps the provider's Revoke function.
func (p *GroupCache) Revoke(s *sessions.SessionState) error {
	return p.provider.Revoke(s)
}

// RefreshAccessToken wraps the provider's RefreshAccessToken function.
func (p *GroupCache) RefreshAccessToken(refreshToken string) (string, time.Duration, error) {
	return p.provider.RefreshAccessToken(refreshToken)
}

// Stop calls the providers stop function.
func (p *GroupCache) Stop() {
	p.provider.Stop()
}
