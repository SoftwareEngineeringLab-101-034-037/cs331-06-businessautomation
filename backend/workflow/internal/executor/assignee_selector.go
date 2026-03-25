package executor

import (
	"math/rand"
	"strings"
	"time"
)

// RoleMemberDirectory abstracts where role members come from.
// This makes assignee lookup swappable without changing executor logic.
type RoleMemberDirectory interface {
	ListMemberIDs(orgID, roleName string) ([]string, error)
}

// TaskAssigneeSelector picks a concrete assignee for a task node.
type TaskAssigneeSelector interface {
	Select(orgID, roleName, preferredUserID string) (string, error)
}

// RandomRoleAssigneeSelector chooses a random member from a role.
// If preferredUserID is provided, it always wins.
type RandomRoleAssigneeSelector struct {
	directory RoleMemberDirectory
	rng       *rand.Rand
}

func NewRandomRoleAssigneeSelector(directory RoleMemberDirectory) *RandomRoleAssigneeSelector {
	return &RandomRoleAssigneeSelector{
		directory: directory,
		rng:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (s *RandomRoleAssigneeSelector) Select(orgID, roleName, preferredUserID string) (string, error) {
	if strings.TrimSpace(preferredUserID) != "" {
		return preferredUserID, nil
	}
	if s.directory == nil {
		return "", nil
	}
	if strings.TrimSpace(roleName) == "" {
		return "", nil
	}
	members, err := s.directory.ListMemberIDs(orgID, roleName)
	if err != nil {
		return "", err
	}
	if len(members) == 0 {
		return "", nil
	}
	return members[s.rng.Intn(len(members))], nil
}

// StaticRoleMemberDirectory keeps role members in-memory and is useful in dev/tests.
// Key shape: orgID -> roleName(lowercased) -> user IDs.
type StaticRoleMemberDirectory struct {
	members map[string]map[string][]string
}

func NewStaticRoleMemberDirectory(members map[string]map[string][]string) *StaticRoleMemberDirectory {
	if members == nil {
		members = map[string]map[string][]string{}
	}
	return &StaticRoleMemberDirectory{members: members}
}

func (d *StaticRoleMemberDirectory) ListMemberIDs(orgID, roleName string) ([]string, error) {
	rolesByOrg, ok := d.members[orgID]
	if !ok {
		return nil, nil
	}
	users, ok := rolesByOrg[strings.ToLower(strings.TrimSpace(roleName))]
	if !ok {
		return nil, nil
	}
	out := make([]string, 0, len(users))
	for _, userID := range users {
		if strings.TrimSpace(userID) == "" {
			continue
		}
		out = append(out, userID)
	}
	return out, nil
}
