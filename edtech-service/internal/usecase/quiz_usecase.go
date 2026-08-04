package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type QuizQuestion struct {
	QuestionText  string   `json:"question_text"`
	Options       []string `json:"options"`
	CorrectAnswer string   `json:"correct_answer"`
}

type SecurityRules struct {
	RequireCamera        bool `json:"require_camera"`
	DetectFaceMovement   bool `json:"detect_face_movement"`
	StrictAppLockdown    bool `json:"strict_app_lockdown"`
	FailOnMinimize       bool `json:"fail_on_minimize"`
}

type CreateQuizRequest struct {
	TutorID       string         `json:"-"`
	Title         string         `json:"title"`
	Description   string         `json:"description"`
	Questions     []QuizQuestion `json:"questions"`
	SecurityRules SecurityRules  `json:"security_rules"`
}

type CreateQuizResponse struct {
	Message string `json:"message"`
	QuizID  string `json:"quiz_id"`
}

type QuizUsecase struct {}

func NewQuizUsecase() *QuizUsecase {
	return &QuizUsecase{}
}

func (u *QuizUsecase) CreateQuiz(ctx context.Context, req CreateQuizRequest) (*CreateQuizResponse, error) {
	if req.Title == "" {
		return nil, errors.New("quiz title is required")
	}
	if len(req.Questions) == 0 {
		return nil, errors.New("quiz must have at least one question")
	}

	// Mock saving to Postgres
	quizID := fmt.Sprintf("quiz_%d", time.Now().Unix())

	return &CreateQuizResponse{
		Message: "Quiz created successfully with strict proctoring enabled.",
		QuizID:  quizID,
	}, nil
}
