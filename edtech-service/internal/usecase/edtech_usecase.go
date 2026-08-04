package usecase

import (
	"context"
	"fmt"
)

type ExecuteCodeRequest struct {
	Language string `json:"language"`
	Code     string `json:"code"`
}

type ExecuteCodeResponse struct {
	Output string `json:"output"`
}

type PurchaseCertificateRequest struct {
	CourseID string `json:"course_id"`
	UserID   string `json:"-"`
}

type PurchaseCertificateResponse struct {
	Status         string `json:"status"`
	CertificateURL string `json:"certificate_url"`
}

type EdtechUsecase struct {}

func NewEdtechUsecase() *EdtechUsecase {
	return &EdtechUsecase{}
}

func (u *EdtechUsecase) ExecuteCode(ctx context.Context, req ExecuteCodeRequest) (*ExecuteCodeResponse, error) {
	// Mock secure code execution response for now
	output := fmt.Sprintf("Running %s code...\nHello World\nProgram exited successfully.", req.Language)
	return &ExecuteCodeResponse{Output: output}, nil
}

func (u *EdtechUsecase) PurchaseCertificate(ctx context.Context, req PurchaseCertificateRequest) (*PurchaseCertificateResponse, error) {
	// Mock wallet interaction to deduct ₦5,000 and return certificate URL
	certURL := fmt.Sprintf("https://lucepay.com/certs/%s_%s.pdf", req.UserID, req.CourseID)
	return &PurchaseCertificateResponse{
		Status:         "success",
		CertificateURL: certURL,
	}, nil
}
