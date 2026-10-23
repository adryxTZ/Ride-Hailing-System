package identity

import (
	"context"
	"fmt"

	"github.com/riverqueue/river"

	"github.com/kalen-limited/hauler-rider/backend/internal/providers/sms"
)

// SendOTPArgs is the job payload. The code travels inside the job table, which
// is acceptable because the table is as protected as otp_codes itself.
type SendOTPArgs struct {
	OTPID string `json:"otp_id"`
	Phone string `json:"phone"`
	Code  string `json:"code"`
}

func (SendOTPArgs) Kind() string { return "identity.send_otp" }

// SendOTPWorker delivers the code through the brand's SMS provider.
type SendOTPWorker struct {
	river.WorkerDefaults[SendOTPArgs]
	Sender    sms.Sender
	BrandName string
}

func (w *SendOTPWorker) Work(ctx context.Context, job *river.Job[SendOTPArgs]) error {
	body := fmt.Sprintf("%s: your sign-in code is %s. It expires in %d minutes.", w.BrandName, job.Args.Code, int(OTPTTL.Minutes()))
	return w.Sender.Send(ctx, job.Args.Phone, body)
}
