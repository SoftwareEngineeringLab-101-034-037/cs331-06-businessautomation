package executor

import (
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/workflow/internal/models"
)

// RoleMemberSelectionStrategy decides which user to pick from role members.
// This keeps assignee selection modular, so the policy can be swapped later.
type RoleMemberSelectionStrategy interface {
	Select(memberIDs []string) string
}

// RandomRoleMemberSelectionStrategy picks a random member from the candidate set.
type RandomRoleMemberSelectionStrategy struct {
	rng *rand.Rand
	mu  sync.Mutex
}

func NewRandomRoleMemberSelectionStrategy() *RandomRoleMemberSelectionStrategy {
	return &RandomRoleMemberSelectionStrategy{
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (s *RandomRoleMemberSelectionStrategy) Select(memberIDs []string) string {
	if len(memberIDs) == 0 {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return memberIDs[s.rng.Intn(len(memberIDs))]
}

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
// preferredUserID is only used as a fallback when roleName is empty.
type RandomRoleAssigneeSelector struct {
	directory RoleMemberDirectory
	strategy  RoleMemberSelectionStrategy
	load      AssigneeTaskLoadProvider
}

// AssigneeTaskLoadProvider provides task history for balancing assignee load.
type AssigneeTaskLoadProvider interface {
	ListTasksByAssignee(orgID, userID string) ([]models.TaskAssignment, error)
}

func NewRandomRoleAssigneeSelector(directory RoleMemberDirectory) *RandomRoleAssigneeSelector {
	return NewRandomRoleAssigneeSelectorWithStrategy(directory, NewRandomRoleMemberSelectionStrategy())
}

// NewBalancedRoleAssigneeSelector picks randomly among the least-assigned
// users in the role, using task history from loadProvider.
func NewBalancedRoleAssigneeSelector(directory RoleMemberDirectory, loadProvider AssigneeTaskLoadProvider) *RandomRoleAssigneeSelector {
	return &RandomRoleAssigneeSelector{
		directory: directory,
		strategy:  NewRandomRoleMemberSelectionStrategy(),
		load:      loadProvider,
	}
}

func NewRandomRoleAssigneeSelectorWithStrategy(directory RoleMemberDirectory, strategy RoleMemberSelectionStrategy) *RandomRoleAssigneeSelector {
	if strategy == nil {
		strategy = NewRandomRoleMemberSelectionStrategy()
	}
	return &RandomRoleAssigneeSelector{
		directory: directory,
		strategy:  strategy,
	}
}

func (s *RandomRoleAssigneeSelector) Select(orgID, roleName, preferredUserID string) (string, error) {
	return s.selectAssignee(orgID, roleName, preferredUserID, "")
}

func (s *RandomRoleAssigneeSelector) SelectWithAuth(orgID, roleName, preferredUserID, authHeader string) (string, error) {
	return s.selectAssignee(orgID, roleName, preferredUserID, authHeader)
}

func (s *RandomRoleAssigneeSelector) selectAssignee(orgID, roleName, preferredUserID, authHeader string) (string, error) {
	trimmedPreferredUserID := strings.TrimSpace(preferredUserID)
	trimmedRoleName := strings.TrimSpace(roleName)
	// Role-based assignment takes precedence. This keeps task routing random
	// across role members even if older workflows still carry assigned_user.
	if trimmedRoleName == "" && trimmedPreferredUserID != "" {
		return trimmedPreferredUserID, nil
	}
	if s.directory == nil {
		return "", nil
	}
	if trimmedRoleName == "" {
		return "", nil
	}
	members, err := s.listMemberIDs(orgID, trimmedRoleName, authHeader)
	if err != nil {
		return "", err
	}
	if len(members) == 0 {
		return "", nil
	}

	balancedCandidates := members
	if s.load != nil {
		leastLoaded, err := s.leastLoadedMembers(orgID, trimmedRoleName, members)
		if err != nil {
			return "", err
		}
		if len(leastLoaded) > 0 {
			balancedCandidates = leastLoaded
		}
	}

	return strings.TrimSpace(s.strategy.Select(balancedCandidates)), nil
}

func (s *RandomRoleAssigneeSelector) leastLoadedMembers(orgID, roleName string, members []string) ([]string, error) {
	if s == nil || s.load == nil || len(members) == 0 {
		return nil, nil
	}

	minCount := -1
	least := make([]string, 0, len(members))
	for _, memberID := range members {
		tasks, err := s.load.ListTasksByAssignee(orgID, memberID)
		if err != nil {
			return nil, err
		}
		count := 0
		for _, task := range tasks {
			if strings.EqualFold(strings.TrimSpace(task.AssignedRole), roleName) {
				count++
			}
		}

		if minCount == -1 || count < minCount {
			minCount = count
			least = []string{memberID}
			continue
		}
		if count == minCount {
			least = append(least, memberID)
		}
	}

	return least, nil
}

func (s *RandomRoleAssigneeSelector) listMemberIDs(orgID, roleName, authHeader string) ([]string, error) {
	type authAwareRoleMemberDirectory interface {
		ListMemberIDsWithAuth(orgID, roleName, authHeader string) ([]string, error)
	}

	if authAware, ok := s.directory.(authAwareRoleMemberDirectory); ok {
		return authAware.ListMemberIDsWithAuth(orgID, roleName, authHeader)
	}
	return s.directory.ListMemberIDs(orgID, roleName)
}

func (s *RandomRoleAssigneeSelector) Directory() RoleMemberDirectory {
	if s == nil {
		return nil
	}
	return s.directory
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
