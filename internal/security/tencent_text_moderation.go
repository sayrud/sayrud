package security

import (
	"context"
	"os"

	"github.com/cockroachdb/errors"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	tms "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/tms/v20201229"
	"github.com/wuhan005/gadget"
)

func TencentTextModeration(ctx context.Context, text string) (bool, error) {
	credential := common.NewCredential(
		os.Getenv("TENCENTCLOUD_CMS_SECRET_ID"),
		os.Getenv("TENCENTCLOUD_CMS_SECRET_KEY"),
	)

	region := os.Getenv("TENCENTCLOUD_CMS_REGION")
	cpf := profile.NewClientProfile()
	client, err := tms.NewClient(credential, region, cpf)
	if err != nil {
		return false, errors.Wrap(err, "new tms client")
	}

	response, err := client.TextModerationWithContext(ctx, &tms.TextModerationRequest{
		Content: common.StringPtr(gadget.Base64Encode(text)),
	})
	if err != nil {
		return false, errors.Wrap(err, "text moderation request")
	}

	return *response.Response.Suggestion != "Block", nil
}
