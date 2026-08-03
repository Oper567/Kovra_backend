package usecase

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"github.com/lucepay-dev/lucepay/backend/engagement-service/internal/domain"
	"github.com/shopspring/decimal"
)

// EngagementUsecase orchestrates referrals, streaks, badges, XP, cashback, leaderboards.
type EngagementUsecase struct {
	referralRepo    ReferralRepository
	streakRepo      StreakRepository
	badgeRepo       BadgeRepository
	xpRepo          XPRepository
	cashbackRepo    CashbackRepository
	leaderboardRepo LeaderboardRepository
	walletClient    WalletClient
	logger          *slog.Logger
}

// ─── Repository Interfaces ──────────────────────────────────

type ReferralRepository interface {
	CreateCode(ctx context.Context, userID, code string, rewardPer decimal.Decimal) (*domain.ReferralCode, error)
	GetCode(ctx context.Context, code string) (*domain.ReferralCode, error)
	GetCodeByUser(ctx context.Context, userID string) (*domain.ReferralCode, error)
	CreateReferral(ctx context.Context, ref *domain.Referral) error
	IncrementUses(ctx context.Context, code string) error
	ListReferrals(ctx context.Context, referrerID string) ([]*domain.Referral, error)
}

type StreakRepository interface {
	GetOrCreate(ctx context.Context, userID string) (*domain.UserStreak, error)
	Update(ctx context.Context, streak *domain.UserStreak) error
	GetRewards(ctx context.Context) ([]*domain.StreakReward, error)
	GetRewardForDay(ctx context.Context, day int) (*domain.StreakReward, error)
}

type BadgeRepository interface {
	GetAll(ctx context.Context) ([]*domain.Badge, error)
	GetBySlug(ctx context.Context, slug string) (*domain.Badge, error)
	AwardBadge(ctx context.Context, userID, badgeID string) error
	GetUserBadges(ctx context.Context, userID string) ([]*domain.UserBadge, error)
	HasBadge(ctx context.Context, userID, badgeID string) (bool, error)
}

type XPRepository interface {
	GetOrCreate(ctx context.Context, userID string) (*domain.UserXP, error)
	AddXP(ctx context.Context, userID string, amount int) (*domain.UserXP, error)
}

type CashbackRepository interface {
	Create(ctx context.Context, entry *domain.CashbackEntry) error
	GetPending(ctx context.Context, userID string) ([]*domain.CashbackEntry, error)
	MarkCredited(ctx context.Context, id, walletTxnID string) error
	GetHistory(ctx context.Context, userID string, limit int) ([]*domain.CashbackEntry, error)
}

type LeaderboardRepository interface {
	Upsert(ctx context.Context, entry *domain.LeaderboardEntry) error
	GetTop(ctx context.Context, category, period string, limit int) ([]*domain.LeaderboardEntry, error)
	GetUserRank(ctx context.Context, userID, category, period string) (*domain.LeaderboardEntry, error)
}

type WalletClient interface {
	CreditWallet(userID, amount, channel, description, idempotencyKey string) (txnID string, err error)
}

func NewEngagementUsecase(
	referralRepo ReferralRepository,
	streakRepo StreakRepository,
	badgeRepo BadgeRepository,
	xpRepo XPRepository,
	cashbackRepo CashbackRepository,
	leaderboardRepo LeaderboardRepository,
	walletClient WalletClient,
	logger *slog.Logger,
) *EngagementUsecase {
	return &EngagementUsecase{
		referralRepo:    referralRepo,
		streakRepo:      streakRepo,
		badgeRepo:       badgeRepo,
		xpRepo:          xpRepo,
		cashbackRepo:    cashbackRepo,
		leaderboardRepo: leaderboardRepo,
		walletClient:    walletClient,
		logger:          logger,
	}
}

// ═══════════════════════════════════════════════════════════
// REFERRAL SYSTEM
// ═══════════════════════════════════════════════════════════

// GetOrCreateReferralCode generates a unique referral code for a user.
func (uc *EngagementUsecase) GetOrCreateReferralCode(ctx context.Context, userID string) (*domain.ReferralCode, error) {
	existing, err := uc.referralRepo.GetCodeByUser(ctx, userID)
	if err == nil && existing != nil {
		return existing, nil
	}

	code := generateReferralCode()
	rewardPer := decimal.NewFromFloat(500.00) // ₦500 per referral

	return uc.referralRepo.CreateCode(ctx, userID, code, rewardPer)
}

// ApplyReferralCode links a new user to a referrer.
func (uc *EngagementUsecase) ApplyReferralCode(ctx context.Context, refereeID, code string) error {
	refCode, err := uc.referralRepo.GetCode(ctx, code)
	if err != nil {
		return domain.ErrReferralCodeNotFound
	}

	if refCode.UserID == refereeID {
		return domain.ErrSelfReferral
	}
	if refCode.Uses >= refCode.MaxUses {
		return domain.ErrReferralCodeExpired
	}

	referral := &domain.Referral{
		ReferrerID:   refCode.UserID,
		RefereeID:    refereeID,
		ReferralCode: code,
		RewardAmount: refCode.RewardPer,
	}

	if err := uc.referralRepo.CreateReferral(ctx, referral); err != nil {
		return err
	}

	uc.referralRepo.IncrementUses(ctx, code)

	// Award cashback to referrer
	uc.awardCashback(ctx, refCode.UserID, "referral", refCode.RewardPer,
		fmt.Sprintf("Referral reward: new user joined with your code %s", code))

	// Award XP
	uc.xpRepo.AddXP(ctx, refCode.UserID, 100)

	uc.logger.InfoContext(ctx, "referral applied",
		slog.String("referrer", refCode.UserID),
		slog.String("referee", refereeID),
		slog.String("code", code),
	)

	return nil
}

func (uc *EngagementUsecase) GetReferralStats(ctx context.Context, userID string) (*domain.ReferralCode, []*domain.Referral, error) {
	code, err := uc.referralRepo.GetCodeByUser(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	referrals, err := uc.referralRepo.ListReferrals(ctx, userID)
	if err != nil {
		return code, nil, err
	}

	return code, referrals, nil
}

// ═══════════════════════════════════════════════════════════
// DAILY LOGIN STREAKS
// ═══════════════════════════════════════════════════════════

type StreakResult struct {
	Streak *domain.UserStreak `json:"streak"`
	Reward *domain.StreakReward `json:"reward,omitempty"`
	XPGained int               `json:"xp_gained"`
}

// RecordLogin updates the user's streak and awards any applicable rewards.
func (uc *EngagementUsecase) RecordLogin(ctx context.Context, userID string) (*StreakResult, error) {
	streak, err := uc.streakRepo.GetOrCreate(ctx, userID)
	if err != nil {
		return nil, err
	}

	today := time.Now().Truncate(24 * time.Hour)
	lastLogin := streak.LastLoginDate.Truncate(24 * time.Hour)

	if today.Equal(lastLogin) {
		// Already logged in today
		return &StreakResult{Streak: streak}, nil
	}

	yesterday := today.Add(-24 * time.Hour)
	if lastLogin.Equal(yesterday) {
		// Consecutive day — extend streak
		streak.CurrentStreak++
	} else {
		// Streak broken — reset
		streak.CurrentStreak = 1
	}

	if streak.CurrentStreak > streak.LongestStreak {
		streak.LongestStreak = streak.CurrentStreak
	}
	streak.LastLoginDate = today
	streak.TotalLogins++

	if err := uc.streakRepo.Update(ctx, streak); err != nil {
		return nil, err
	}

	result := &StreakResult{Streak: streak, XPGained: 10}
	uc.xpRepo.AddXP(ctx, userID, 10) // 10 XP per login

	// Check for streak milestone rewards
	reward, err := uc.streakRepo.GetRewardForDay(ctx, streak.CurrentStreak)
	if err == nil && reward != nil {
		result.Reward = reward
		uc.awardCashback(ctx, userID, "streak", reward.RewardValue, reward.Description)
		result.XPGained += 50 // Bonus XP for milestone
		uc.xpRepo.AddXP(ctx, userID, 50)

		uc.logger.InfoContext(ctx, "streak milestone reached",
			slog.String("user_id", userID),
			slog.Int("day", streak.CurrentStreak),
			slog.String("reward", reward.RewardValue.String()),
		)
	}

	return result, nil
}

// ═══════════════════════════════════════════════════════════
// BADGES & ACHIEVEMENTS
// ═══════════════════════════════════════════════════════════

func (uc *EngagementUsecase) AwardBadge(ctx context.Context, userID, badgeSlug string) (*domain.Badge, error) {
	badge, err := uc.badgeRepo.GetBySlug(ctx, badgeSlug)
	if err != nil {
		return nil, domain.ErrBadgeNotFound
	}

	has, _ := uc.badgeRepo.HasBadge(ctx, userID, badge.ID)
	if has {
		return badge, domain.ErrBadgeAlreadyEarned
	}

	if err := uc.badgeRepo.AwardBadge(ctx, userID, badge.ID); err != nil {
		return nil, err
	}

	// Award XP for the badge
	uc.xpRepo.AddXP(ctx, userID, badge.XPReward)

	uc.logger.InfoContext(ctx, "badge awarded",
		slog.String("user_id", userID),
		slog.String("badge", badgeSlug),
		slog.Int("xp", badge.XPReward),
	)

	return badge, nil
}

func (uc *EngagementUsecase) GetUserBadges(ctx context.Context, userID string) ([]*domain.UserBadge, error) {
	return uc.badgeRepo.GetUserBadges(ctx, userID)
}

func (uc *EngagementUsecase) GetAllBadges(ctx context.Context) ([]*domain.Badge, error) {
	return uc.badgeRepo.GetAll(ctx)
}

// ═══════════════════════════════════════════════════════════
// XP & LEVELS
// ═══════════════════════════════════════════════════════════

func (uc *EngagementUsecase) GetUserXP(ctx context.Context, userID string) (*domain.UserXP, error) {
	return uc.xpRepo.GetOrCreate(ctx, userID)
}

// ═══════════════════════════════════════════════════════════
// CASHBACK
// ═══════════════════════════════════════════════════════════

func (uc *EngagementUsecase) awardCashback(ctx context.Context, userID, source string, amount decimal.Decimal, desc string) {
	entry := &domain.CashbackEntry{
		UserID:      userID,
		Source:      source,
		Amount:      amount,
		Description: desc,
	}

	if err := uc.cashbackRepo.Create(ctx, entry); err != nil {
		uc.logger.ErrorContext(ctx, "cashback save failed", slog.String("error", err.Error()))
		return
	}

	// Auto-credit to wallet
	idempKey := fmt.Sprintf("cashback-%s-%s-%d", userID, source, time.Now().UnixMilli())
	txnID, err := uc.walletClient.CreditWallet(
		userID, amount.String(), "WALLET_FUND",
		fmt.Sprintf("Luce Pay Cashback: %s", desc), idempKey,
	)
	if err != nil {
		uc.logger.ErrorContext(ctx, "cashback wallet credit failed",
			slog.String("user_id", userID),
			slog.String("error", err.Error()),
		)
		return
	}

	uc.cashbackRepo.MarkCredited(ctx, entry.ID, txnID)
}

func (uc *EngagementUsecase) GetCashbackHistory(ctx context.Context, userID string, limit int) ([]*domain.CashbackEntry, error) {
	return uc.cashbackRepo.GetHistory(ctx, userID, limit)
}

// ═══════════════════════════════════════════════════════════
// LEADERBOARD
// ═══════════════════════════════════════════════════════════

func (uc *EngagementUsecase) GetLeaderboard(ctx context.Context, category, period string, limit int) ([]*domain.LeaderboardEntry, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return uc.leaderboardRepo.GetTop(ctx, category, period, limit)
}

func (uc *EngagementUsecase) GetUserRank(ctx context.Context, userID, category, period string) (*domain.LeaderboardEntry, error) {
	return uc.leaderboardRepo.GetUserRank(ctx, userID, category, period)
}

// ─── Helpers ─────────────────────────────────────────────────

func generateReferralCode() string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // No ambiguous chars
	code := make([]byte, 8)
	for i := range code {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		code[i] = charset[n.Int64()]
	}
	return string(code)
}
