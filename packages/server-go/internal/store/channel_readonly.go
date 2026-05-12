// Package store — chn_15_readonly.go: CHN-15 channel readonly helpers.
//
// Spec: docs/implementation/modules/chn-15-spec.md §1, CHN-15.1 split.
//
// Behaviour: readonly is channel-wide state stored on the **creator's**
// user_channel_layout.collapsed row at bit 4 (=16). All members read
// the same single row to determine the channel's readonly state; only
// the creator can mutate it (handler-layer ACL).
//
// Explicit non-goals (chn-15-spec.md §0 stance ①+⑤):
//   - Keep bit 4 writes centralized here; handler / api layers must not
//     update it with direct SQL.
//   - GetChannelReadonly reads the single user_channel_layout row for
//     channel.CreatedBy, and must not read non-creator rows.
//   - SetChannelReadonly wraps SetMuteBit, reusing the #550 single source of
//     truth for bitmask writes.
package store

import (
	"errors"
)

// readonlyBitInternal mirrors api.ReadonlyBit (=16). Kept as an
// unexported const here so the store package doesn't import internal/api.
// Drift is caught by api.TestCHN151_ReadonlyBit_ByteIdentical which pins
// the literal 16 from the api side.
const readonlyBitInternal = 16

// GetChannelReadonly reports whether channelID is currently flagged as
// readonly. Reads the **creator's** user_channel_layout.collapsed row
// (channel-wide state via the creator's single-row source of truth, stance ⑤).
//
// Returns (false, nil) when the channel exists but the creator has no
// layout row yet (NULL = no bits set = not readonly).
func (s *Store) GetChannelReadonly(channelID string) (bool, error) {
	if channelID == "" {
		return false, errors.New("channelID required")
	}
	var ch Channel
	if err := s.db.Select("id", "created_by").
		Where("id = ?", channelID).First(&ch).Error; err != nil {
		return false, err
	}
	var collapsed int64
	if err := s.db.Raw(`SELECT COALESCE(collapsed, 0)
		FROM user_channel_layout
		WHERE user_id = ? AND channel_id = ?`,
		ch.CreatedBy, channelID).Row().Scan(&collapsed); err != nil {
		// no row → not readonly
		return false, nil
	}
	return collapsed&int64(readonlyBitInternal) != 0, nil
}

// SetChannelReadonly toggles bit 4 of the **creator's**
// user_channel_layout.collapsed row for channelID. Wraps SetMuteBit
// for the bitmask write — preserves bits 0/1/2-3.
//
// Caller MUST validate that the requesting user is channel.CreatedBy
// before calling (handler-layer ACL, stance ②).
func (s *Store) SetChannelReadonly(channelID string, readonly bool) (int64, error) {
	if channelID == "" {
		return 0, errors.New("channelID required")
	}
	var ch Channel
	if err := s.db.Select("id", "created_by").
		Where("id = ?", channelID).First(&ch).Error; err != nil {
		return 0, err
	}
	return s.SetMuteBit(ch.CreatedBy, channelID, int64(readonlyBitInternal), readonly)
}
