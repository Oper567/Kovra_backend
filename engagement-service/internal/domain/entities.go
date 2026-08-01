package domain

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

// ─── Referral System ────────────────────────────────────────

type ReferralCode struct {
	ID        string          `json:"id"`
	UserID    string          `json:"user_id"`
	Code      string          `json:"code"`
	Uses      int             `json:"uses"`
	MaxUses   int             `json:"max_uses"`
	RewardPer decimal.Decimal `json:"reward_per"`
	IsActive  bool            `json:"is_active"`
	CreatedAt time.Time       `json:"created_at"`
}

type Referral struct {
	ID            string          `json:"id"`
	ReferrerID    string          `json:"referrer_id"`
	RefereeID     string          `json:"referee_id"`
	ReferralCode  string          `json:"referral_code"`
	RewardAmount  decimal.Decimal `json:"reward_amount"`
	RewardPaid    bool            `json:"reward_paid"`
	RefereeAction string          `json:"referee_action"`
	CreatedAt     time.Time       `json:"created_at"`
}

// ─── Streaks ────────────────────────────────────────────────

type UserStreak struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	CurrentStreak  int       `json:"current_streak"`
	LongestStreak  int       `json:"longest_streak"`
	LastLoginDate  time.Time `json:"last_login_date"`
	TotalLogins    int       `json:"total_logins"`
}

type StreakReward struct {
	StreakDay    int             `json:"streak_day"`
	RewardType  string          `json:"reward_type"`
	RewardValue decimal.Decimal `json:"reward_value"`
	BadgeName   string          `json:"badge_name"`
	Description string          `json:"description"`
}

// ─── Badges & XP ────────────────────────────────────────────

type Badge struct {
	ID          string         `json:"id"`
	Slug        string         `json:"slug"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	IconURL     string         `json:"icon_url"`
	Category    string         `json:"category"`
	Criteria    map[string]any `json:"criteria"`
	XPReward    int            `json:"xp_reward"`
}

type UserBadge struct {
	BadgeID  string    `json:"badge_id"`
	Badge    *Badge    `json:"badge"`
	EarnedAt time.Time `json:"earned_at"`
}

type UserXP struct {
	UserID   string `json:"user_id"`
	TotalXP  int    `json:"total_xp"`
	Level    int    `json:"level"`
	Tier     string `json:"tier"` // BRONZE, SILVER, GOLD, PLATINUM, DIAMOND
}

// ─── Cashback ───────────────────────────────────────────────

type CashbackEntry struct {
	ID          string          `json:"id"`
	UserID      string          `json:"user_id"`
	Source      string          `json:"source"`
	Amount      decimal.Decimal `json:"amount"`
	Description string          `json:"description"`
	IsCredited  bool            `json:"is_credited"`
	CreatedAt   time.Time       `json:"created_at"`
}

// ─── Leaderboard ────────────────────────────────────────────

type LeaderboardEntry struct {
	UserID   string `json:"user_id"`
	Username string `json:"username,omitempty"`
	Category string `json:"category"`
	Score    int    `json:"score"`
	Rank     int    `json:"rank"`
	Period   string `json:"period"`
}

// ─── Errors ──────────────────────────────────────────────────

var (
	ErrReferralCodeNotFound   = errors.New("referral code not found")
	ErrReferralCodeExpired    = errors.New("referral code has reached max uses")
	ErrSelfReferral           = errors.New("cannot refer yourself")
	ErrAlreadyReferred        = errors.New("user has already been referred")
	ErrStreakNotFound          = errors.New("streak record not found")
	ErrBadgeAlreadyEarned     = errors.New("badge already earned")
	ErrBadgeNotFound          = errors.New("badge not found")
	ErrNoCashbackAvailable    = errors.New("no cashback available")
)
