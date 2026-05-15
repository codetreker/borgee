package store

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

const (
	RequireMentionPolicyInherit = "inherit"
	RequireMentionPolicyOn      = "on"
	RequireMentionPolicyOff     = "off"
)

var (
	ErrInvalidRequireMentionPolicy         = errors.New("invalid require mention policy")
	ErrRequireMentionPolicyTargetNotMember = errors.New("require mention target is not a channel member")
	ErrRequireMentionPolicyTargetNotAgent  = errors.New("require mention target must be an agent")
	ErrRequireMentionPolicyOwnerCeiling    = errors.New("require mention policy off exceeds owner authorization")
)

type ChannelMemberRequireMentionState struct {
	ChannelID               string `json:"channel_id"`
	UserID                  string `json:"user_id"`
	RequireMentionPolicy    string `json:"require_mention_policy"`
	EffectiveRequireMention bool   `json:"effective_require_mention"`
}

func NormalizeRequireMentionPolicy(policy string) (string, error) {
	switch policy {
	case "", RequireMentionPolicyInherit:
		return RequireMentionPolicyInherit, nil
	case RequireMentionPolicyOn:
		return RequireMentionPolicyOn, nil
	case RequireMentionPolicyOff:
		return RequireMentionPolicyOff, nil
	default:
		return "", ErrInvalidRequireMentionPolicy
	}
}

func (s *Store) GetChannelMemberRequireMentionState(channelID, userID string) (ChannelMemberRequireMentionState, error) {
	row, err := s.loadRequireMentionPolicyRow(channelID, userID)
	if err != nil {
		return ChannelMemberRequireMentionState{}, err
	}
	if row.Role != "agent" {
		return ChannelMemberRequireMentionState{}, ErrRequireMentionPolicyTargetNotAgent
	}
	policy, err := NormalizeRequireMentionPolicy(row.RequireMentionPolicy)
	if err != nil {
		policy = RequireMentionPolicyInherit
	}
	return ChannelMemberRequireMentionState{
		ChannelID:               channelID,
		UserID:                  userID,
		RequireMentionPolicy:    policy,
		EffectiveRequireMention: effectiveRequireMention(policy, row.RequireMention),
	}, nil
}

func (s *Store) SetChannelMemberRequireMentionPolicy(channelID, userID, policy string) (ChannelMemberRequireMentionState, error) {
	normalized, err := NormalizeRequireMentionPolicy(policy)
	if err != nil {
		return ChannelMemberRequireMentionState{}, err
	}
	row, err := s.loadRequireMentionPolicyRow(channelID, userID)
	if err != nil {
		return ChannelMemberRequireMentionState{}, err
	}
	if row.Role != "agent" {
		return ChannelMemberRequireMentionState{}, ErrRequireMentionPolicyTargetNotAgent
	}
	if normalized == RequireMentionPolicyOff && row.RequireMention {
		return ChannelMemberRequireMentionState{}, ErrRequireMentionPolicyOwnerCeiling
	}
	if err := s.db.Model(&ChannelMember{}).
		Where("channel_id = ? AND user_id = ?", channelID, userID).
		Update("require_mention_policy", normalized).Error; err != nil {
		return ChannelMemberRequireMentionState{}, err
	}
	return ChannelMemberRequireMentionState{
		ChannelID:               channelID,
		UserID:                  userID,
		RequireMentionPolicy:    normalized,
		EffectiveRequireMention: effectiveRequireMention(normalized, row.RequireMention),
	}, nil
}

func (s *Store) ListChannelAgentsAllowedWithoutMention(channelID, senderID string, excludedIDs []string) ([]string, error) {
	excluded := make(map[string]struct{}, len(excludedIDs)+1)
	if senderID != "" {
		excluded[senderID] = struct{}{}
	}
	for _, id := range excludedIDs {
		if id != "" {
			excluded[id] = struct{}{}
		}
	}

	var rows []requireMentionPolicyRow
	if err := s.db.Table("channel_members cm").
		Select("cm.channel_id, cm.user_id, COALESCE(cm.require_mention_policy, '') AS require_mention_policy, u.role, u.require_mention").
		Joins("JOIN users u ON u.id = cm.user_id").
		Where("cm.channel_id = ? AND u.role = ? AND u.deleted_at IS NULL", channelID, "agent").
		Order("cm.joined_at ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		if _, skip := excluded[row.UserID]; skip {
			continue
		}
		policy, err := NormalizeRequireMentionPolicy(row.RequireMentionPolicy)
		if err != nil {
			continue
		}
		if !effectiveRequireMention(policy, row.RequireMention) {
			out = append(out, row.UserID)
		}
	}
	return out, nil
}

func (s *Store) ListEveryoneMentionTargets(channelID, senderID string) ([]string, error) {
	var rows []struct {
		UserID string `gorm:"column:user_id"`
	}
	if err := s.db.Table("channel_members cm").
		Select("cm.user_id").
		Joins("JOIN users u ON u.id = cm.user_id AND u.deleted_at IS NULL").
		Where("cm.channel_id = ? AND cm.user_id <> ?", channelID, senderID).
		Order("cm.joined_at ASC, cm.user_id ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.UserID != "" {
			out = append(out, row.UserID)
		}
	}
	return out, nil
}

type requireMentionPolicyRow struct {
	ChannelID            string `gorm:"column:channel_id"`
	UserID               string `gorm:"column:user_id"`
	RequireMentionPolicy string `gorm:"column:require_mention_policy"`
	Role                 string `gorm:"column:role"`
	RequireMention       bool   `gorm:"column:require_mention"`
}

func (s *Store) loadRequireMentionPolicyRow(channelID, userID string) (requireMentionPolicyRow, error) {
	var row requireMentionPolicyRow
	err := s.db.Table("channel_members cm").
		Select("cm.channel_id, cm.user_id, COALESCE(cm.require_mention_policy, '') AS require_mention_policy, u.role, u.require_mention").
		Joins("JOIN users u ON u.id = cm.user_id AND u.deleted_at IS NULL").
		Where("cm.channel_id = ? AND cm.user_id = ?", channelID, userID).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return requireMentionPolicyRow{}, fmt.Errorf("%w: %v", ErrRequireMentionPolicyTargetNotMember, err)
		}
		return requireMentionPolicyRow{}, fmt.Errorf("load require mention policy row: %w", err)
	}
	return row, nil
}

func effectiveRequireMention(policy string, globalRequireMention bool) bool {
	switch policy {
	case RequireMentionPolicyOn:
		return true
	case RequireMentionPolicyOff:
		return globalRequireMention
	default:
		return globalRequireMention
	}
}
