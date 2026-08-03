package postgres

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/lucepay-dev/lucepay/backend/ai-service/internal/domain"
	"github.com/lucepay-dev/lucepay/backend/ai-service/internal/usecase"
)

type insightRepo struct {
	db *sql.DB
}

func NewInsightRepository(db *sql.DB) usecase.InsightRepository {
	return &insightRepo{db: db}
}

func (r *insightRepo) Save(ctx context.Context, insight *domain.SpendingInsight) error {
	query := `
		INSERT INTO ai_spending_insights (user_id, period, insight_type, title, body, data, generated_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, is_read`

	dataJSON, _ := json.Marshal(insight.Data)

	err := r.db.QueryRowContext(ctx, query,
		insight.UserID, insight.Period, insight.InsightType, insight.Title, insight.Body, dataJSON, insight.GeneratedAt, insight.ExpiresAt,
	).Scan(&insight.ID, &insight.IsRead)

	return err
}

func (r *insightRepo) ListByUser(ctx context.Context, userID string, limit int) ([]*domain.SpendingInsight, error) {
	query := `
		SELECT id, user_id, period, insight_type, title, body, data, is_read, generated_at, expires_at
		FROM ai_spending_insights
		WHERE user_id = $1 AND expires_at > NOW()
		ORDER BY generated_at DESC
		LIMIT $2`

	rows, err := r.db.QueryContext(ctx, query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var insights []*domain.SpendingInsight
	for rows.Next() {
		var i domain.SpendingInsight
		var dataJSON []byte
		if err := rows.Scan(
			&i.ID, &i.UserID, &i.Period, &i.InsightType, &i.Title, &i.Body, &dataJSON, &i.IsRead, &i.GeneratedAt, &i.ExpiresAt,
		); err != nil {
			return nil, err
		}
		json.Unmarshal(dataJSON, &i.Data)
		insights = append(insights, &i)
	}
	return insights, nil
}

func (r *insightRepo) MarkRead(ctx context.Context, insightID string) error {
	query := `UPDATE ai_spending_insights SET is_read = TRUE WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, insightID)
	return err
}
