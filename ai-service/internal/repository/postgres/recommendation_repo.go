package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/kovra-dev/kovra/backend/ai-service/internal/domain"
	"github.com/kovra-dev/kovra/backend/ai-service/internal/usecase"
)

type recommendationRepo struct {
	db *sql.DB
}

func NewRecommendationRepository(db *sql.DB) usecase.RecommendationRepository {
	return &recommendationRepo{db: db}
}

func (r *recommendationRepo) SaveBatch(ctx context.Context, recs []*domain.Recommendation) error {
	if len(recs) == 0 {
		return nil
	}

	query := `
		INSERT INTO ai_recommendations (user_id, rec_type, item_id, score, reason, created_at)
		VALUES `

	var values []interface{}
	var placeholders []string

	for i, rec := range recs {
		p := i * 6
		placeholders = append(placeholders, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d)", p+1, p+2, p+3, p+4, p+5, p+6))
		values = append(values, rec.UserID, rec.RecType, rec.ItemID, rec.Score, rec.Reason, rec.CreatedAt)
	}

	query += strings.Join(placeholders, ", ")
	_, err := r.db.ExecContext(ctx, query, values...)
	return err
}

func (r *recommendationRepo) ListByUser(ctx context.Context, userID, recType string, limit int) ([]*domain.Recommendation, error) {
	query := `
		SELECT id, user_id, rec_type, item_id, score, reason, is_dismissed, created_at
		FROM ai_recommendations
		WHERE user_id = $1 AND rec_type = $2 AND is_dismissed = FALSE
		ORDER BY score DESC, created_at DESC
		LIMIT $3`

	rows, err := r.db.QueryContext(ctx, query, userID, recType, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var recs []*domain.Recommendation
	for rows.Next() {
		var rec domain.Recommendation
		if err := rows.Scan(
			&rec.ID, &rec.UserID, &rec.RecType, &rec.ItemID, &rec.Score, &rec.Reason, &rec.IsDismissed, &rec.CreatedAt,
		); err != nil {
			return nil, err
		}
		recs = append(recs, &rec)
	}
	return recs, nil
}

func (r *recommendationRepo) Dismiss(ctx context.Context, recID string) error {
	query := `UPDATE ai_recommendations SET is_dismissed = TRUE WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, recID)
	return err
}
